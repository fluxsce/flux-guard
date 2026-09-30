package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
	"fluxguard/server/job/exec"
	"fluxguard/server/job/ext/archive"
	"fluxguard/server/job/policy"
	"fluxguard/server/job/registry"
	"fluxguard/server/job/status"
	"fluxguard/server/store"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	accessdao "fluxguard/server/web/middleware/dao"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0025/dao"
	"fluxguard/server/web/views/fg0025/models"

	dbx "github.com/fluxsce/dbx/db"
	"github.com/gin-gonic/gin"
)

// JobController 系统任务管理。
type JobController struct {
	access   *accessdao.Access
	settings *dao.SettingDAO
}

// NewJobController 创建任务管理控制器。
func NewJobController(db *dbx.DB) *JobController {
	return &JobController{access: accessdao.Open(db), settings: dao.NewSettingDAO(db)}
}

// QueryJobs 返回任务目录、本租户策略摘要与最近执行。
func (c *JobController) QueryJobs(g *gin.Context) {
	p := middleware.Principal(g)
	editable := c.access.Allow(p.TenantID, p.UserID, "fg0025:edit")
	items := make([]models.JobRow, 0, len(catalog.Specs()))
	for _, spec := range catalog.Specs() {
		tenantID := p.TenantID
		if spec.Scope == catalog.ScopeSystem {
			tenantID = catalog.SystemTenant
		}
		row := models.JobRow{
			Code:        spec.Code,
			Title:       spec.Title,
			Description: spec.Description,
			Kind:        spec.Kind,
			Source:      spec.Source,
			Scope:       spec.Scope,
		}
		sched := policy.DecodeSchedule(policy.Get(tenantID, spec.Code))
		row.Enabled = sched.Enabled
		row.Mode = sched.Mode
		row.IntervalMinutes = sched.IntervalMinutes
		row.RunAt = sched.RunAt
		row.Cron = sched.Cron
		row.TimeoutSeconds = sched.TimeoutSeconds
		row.RetryCount = sched.RetryCount
		row.Deletable = spec.Source == catalog.SourceExternal
		st, hasStatus := status.Get(tenantID, spec.Code)
		row.NextStart = nextStartText(sched, st, hasStatus)
		if hasStatus {
			row.LastRunAt = store.FormatTime(st.LastEnd)
			row.LastOK = st.OK
			row.LastDurationMs = st.DurationMs
			row.LastResult = string(st.Result)
			row.LastError = st.ErrorText
			row.LastTrigger = st.Trigger
			row.RanAtLeast = st.RanAtLeast
		}
		items = append(items, row)
	}
	kinds := jobKinds()
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Kind < kinds[j].Kind })
	httpx.WriteOK(g, models.JobList{Editable: editable, Items: items, Kinds: kinds})
}

// GetJob 返回单个任务详情。按扩展 Kind 分发可配策略。
func (c *JobController) GetJob(g *gin.Context) {
	p := middleware.Principal(g)
	code := strings.TrimSpace(g.Query("code"))
	spec, ok := catalog.ByCode(code)
	if !ok {
		httpx.WriteErr(g, http.StatusBadRequest, "未知的任务编码")
		return
	}
	editable := c.access.Allow(p.TenantID, p.UserID, "fg0025:edit")
	httpx.WriteOK(g, c.archiveView(g, tenantOf(spec, p.TenantID), spec, editable))
}

// SaveArchive 保存数据归档任务策略。
func (c *JobController) SaveArchive(g *gin.Context) {
	var req models.SaveArchiveRequest
	if !request.Bind(g, &req) {
		return
	}
	p := middleware.Principal(g)
	audit.Note(g, "", "JOB", archive.Code, "数据归档", "修改数据归档任务")
	value := models.ArchiveFrom(req)
	if err := archive.Validate(value); err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	raw, err := archive.Encode(value)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存任务失败")
		return
	}
	version, err := c.settings.Save(g.Request.Context(), policy.Group(archive.Code), req.CurrentVersion, string(raw))
	if errors.Is(err, dao.ErrConflict) {
		httpx.WriteErr(g, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存任务失败")
		return
	}
	archive.Put(p.TenantID, value)
	status.Touch(p.TenantID, archive.Code, "数据归档", value.Enabled)
	audit.Note(g, "", "JOB", archive.Code, "数据归档", fmt.Sprintf(
		"修改数据归档：启用=%v 方式=%s 间隔=%d 时间=%s cron=%s 超时=%d 重试=%d 审计=%d 运维=%d 预警=%d 样本=%d",
		value.Enabled, value.Mode, value.IntervalMinutes, value.RunAt, value.Cron, value.TimeoutSeconds, value.RetryCount,
		value.Params.AuditLogDays, value.Params.OpsLogDays, value.Params.AlertDays, value.Params.SampleDays,
	))
	spec, _ := catalog.ByCode(archive.Code)
	httpx.WriteOK(g, c.archiveFrom(value, version, true, spec, p.TenantID, raw))
}

// Run 按当前策略立即执行一次，并把结果写回状态。
func (c *JobController) Run(g *gin.Context) {
	var req models.RunJobRequest
	if !request.Bind(g, &req) {
		return
	}
	spec, ok := catalog.ByCode(req.Code)
	if !ok {
		httpx.WriteErr(g, http.StatusBadRequest, "未知的任务编码")
		return
	}
	p := middleware.Principal(g)
	tenantID := p.TenantID
	if spec.Scope == catalog.ScopeSystem {
		tenantID = catalog.SystemTenant
	}
	audit.Note(g, "", "JOB", spec.Code, spec.Title, "立即执行 "+spec.Title)
	if _, err := exec.Run(g.Request.Context(), spec.Code, tenantID, contract.TriggerManual); err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, c.archiveView(g, tenantID, spec, true))
}

// QueryLogs 返回某任务最近的执行日志，新的在前。
func (c *JobController) QueryLogs(g *gin.Context) {
	p := middleware.Principal(g)
	code := strings.TrimSpace(g.Query("code"))
	spec, ok := catalog.ByCode(code)
	if !ok {
		httpx.WriteErr(g, http.StatusBadRequest, "未知的任务编码")
		return
	}
	logs, err := status.Recent(tenantOf(spec, p.TenantID), spec.Code, 100)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "读取执行日志失败")
		return
	}
	items := make([]models.JobLogRow, 0, len(logs))
	for _, item := range logs {
		items = append(items, models.JobLogRow{
			LogID:      item.LogID,
			Trigger:    item.Trigger,
			StartTime:  store.FormatTime(item.Start),
			EndTime:    store.FormatTime(item.End),
			DurationMs: item.DurationMs,
			OK:         item.OK,
			Result:     string(item.Result),
			ErrorText:  item.ErrorText,
		})
	}
	httpx.WriteOK(g, models.JobLogList{Code: spec.Code, Title: spec.Title, Items: items})
}

// jobKinds 合并已登记执行器和目录里的内置任务，避免内置种类在新增时丢失。
func jobKinds() []models.JobKind {
	seen := map[string]struct{}{}
	kinds := make([]models.JobKind, 0)
	add := func(kind, title, description, source string) {
		kind = strings.TrimSpace(kind)
		if kind == "" || source == catalog.SourceExternal {
			return
		}
		if _, ok := seen[kind]; ok {
			return
		}
		seen[kind] = struct{}{}
		kinds = append(kinds, models.JobKind{Kind: kind, Title: title, Description: description})
	}
	for _, kind := range registry.Kinds() {
		add(kind.Kind, kind.Title, kind.Description, catalog.SourceBuiltin)
	}
	for _, spec := range catalog.Specs() {
		add(spec.Kind, spec.Title, spec.Description, spec.Source)
	}
	return kinds
}

func (c *JobController) archiveView(g *gin.Context, tenantID string, spec catalog.Spec, editable bool) models.ArchiveJob {
	content, version, ok, err := c.settings.Get(g.Request.Context(), tenantID, policy.Group(spec.Code))
	var value archive.Policy
	var stored json.RawMessage
	if err == nil && ok {
		stored = json.RawMessage(content)
		value = archive.Parse(stored)
		if spec.Code == archive.Code {
			archive.Put(tenantID, value)
		}
	} else {
		stored = policy.Get(tenantID, spec.Code)
		value = archive.Parse(stored)
	}
	return c.archiveFrom(value, version, editable, spec, tenantID, stored)
}

func (c *JobController) archiveFrom(value archive.Policy, version int, editable bool, spec catalog.Spec, tenantID string, stored json.RawMessage) models.ArchiveJob {
	view := models.ArchiveJob{
		Code:            spec.Code,
		Title:           spec.Title,
		Description:     spec.Description,
		Kind:            spec.Kind,
		Source:          spec.Source,
		Scope:           spec.Scope,
		Enabled:         value.Enabled,
		Mode:            value.Mode,
		IntervalMinutes: value.IntervalMinutes,
		RunAt:           value.RunAt,
		Cron:            value.Cron,
		TimeoutSeconds:  value.TimeoutSeconds,
		RetryCount:      value.RetryCount,
		AuditLogDays:    value.Params.AuditLogDays,
		OpsLogDays:      value.Params.OpsLogDays,
		AlertDays:       value.Params.AlertDays,
		SampleDays:      value.Params.SampleDays,
		Params:          prettyParams(stored),
		CurrentVersion:  version,
		Editable:        editable,
		Deletable:       spec.Source == catalog.SourceExternal,
	}
	sched := policy.DecodeSchedule(policy.Get(tenantID, spec.Code))
	st, hasStatus := status.Get(tenantID, spec.Code)
	view.NextStart = nextStartText(sched, st, hasStatus)
	if hasStatus {
		view.LastRunAt = store.FormatTime(st.LastEnd)
		view.LastOK = st.OK
		view.LastDurationMs = st.DurationMs
		view.LastResult = string(st.Result)
		view.LastError = st.ErrorText
		view.LastTrigger = st.Trigger
		view.RanAtLeast = st.RanAtLeast
	}
	return view
}

func nextStartText(sched policy.Schedule, st status.Status, hasStatus bool) string {
	if !sched.Enabled {
		return ""
	}
	var last time.Time
	if hasStatus && st.RanAtLeast {
		last = st.LastStart
		if last.IsZero() {
			last = st.LastEnd
		}
	}
	at, ok := policy.NextStart(sched, last, time.Now().UTC())
	if !ok {
		return ""
	}
	return store.FormatTime(at)
}

func prettyParams(raw json.RawMessage) string {
	out, err := json.MarshalIndent(policy.Params(raw), "", "  ")
	if err != nil || len(out) == 0 {
		return "{}"
	}
	return string(out)
}
