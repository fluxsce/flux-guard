package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
	"fluxguard/server/job/ext/archive"
	"fluxguard/server/job/policy"
	"fluxguard/server/job/registry"
	"fluxguard/server/job/status"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0025/dao"
	"fluxguard/server/web/views/fg0025/models"

	"github.com/gin-gonic/gin"
)

var jobCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func tenantOf(spec catalog.Spec, tenantID string) string {
	if spec.Scope == catalog.ScopeSystem {
		return catalog.SystemTenant
	}
	return tenantID
}

// Save 新建或修改任务。新建时种类必须是已登记的实现，调度按该实现调用。
func (c *JobController) Save(g *gin.Context) {
	var req models.SaveJobRequest
	if !request.Bind(g, &req) {
		return
	}
	code := strings.TrimSpace(req.Code)
	title := strings.TrimSpace(req.Title)
	spec, exists := catalog.ByCode(code)
	if !exists {
		if !jobCodePattern.MatchString(code) {
			httpx.WriteErr(g, http.StatusBadRequest, "任务编码须为字母开头，仅含小写字母、数字、下划线或连字符")
			return
		}
		if title == "" {
			httpx.WriteErr(g, http.StatusBadRequest, "请填写任务名称")
			return
		}
		if strings.TrimSpace(req.Kind) == "" {
			httpx.WriteErr(g, http.StatusBadRequest, "请选择任务种类")
			return
		}
	}
	value := policyFrom(req)
	paramsRaw, err := policy.ParseObject(req.Params)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	kindNow := spec.Kind
	if kindNow == "" {
		kindNow = strings.TrimSpace(req.Kind)
	}
	if kindNow == archive.Kind && strings.TrimSpace(req.Params) != "" {
		value.Params = archive.ParseParams(paramsRaw)
	}
	if err := validateJob(spec, req.Kind, value); err != nil && exists {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	if !exists {
		if err := validateJob(catalog.Spec{Kind: req.Kind}, req.Kind, value); err != nil {
			httpx.WriteErr(g, http.StatusBadRequest, err.Error())
			return
		}
		desc := strings.TrimSpace(req.Description)
		if desc == "" {
			desc = kindDescription(req.Kind)
		}
		if err := registry.Bind(contract.Meta{
			Code:        code,
			Title:       title,
			Description: desc,
			Kind:        strings.TrimSpace(req.Kind),
			Source:      catalog.SourceExternal,
			Scope:       catalog.ScopeTenant,
		}); err != nil {
			httpx.WriteErr(g, http.StatusBadRequest, err.Error())
			return
		}
		spec, _ = catalog.ByCode(code)
	} else if spec.Source == catalog.SourceExternal {
		if title == "" {
			title = spec.Title
		}
		desc := strings.TrimSpace(req.Description)
		if desc == "" {
			desc = spec.Description
		}
		if err := registry.Bind(contract.Meta{
			Code:        spec.Code,
			Title:       title,
			Description: desc,
			Kind:        spec.Kind,
			Source:      catalog.SourceExternal,
			Scope:       spec.Scope,
		}); err != nil {
			httpx.WriteErr(g, http.StatusBadRequest, err.Error())
			return
		}
		spec, _ = catalog.ByCode(code)
	}
	if spec.Source == catalog.SourceExternal {
		value.Title = spec.Title
		value.Description = spec.Description
		value.Kind = spec.Kind
		value.Source = catalog.SourceExternal
		value.Scope = spec.Scope
	}
	raw, err := encodeJob(spec, value, req.Params)
	if err != nil {
		if !exists {
			_ = registry.Unregister(code)
		}
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	p := middleware.Principal(g)
	version, err := c.settings.Save(g.Request.Context(), policy.Group(spec.Code), req.CurrentVersion, string(raw))
	if err != nil {
		if !exists {
			_ = registry.Unregister(code)
		}
		writeSaveErr(g, err)
		return
	}
	tenantID := tenantOf(spec, p.TenantID)
	policy.Put(tenantID, spec.Code, raw)
	if spec.Code == archive.Code {
		archive.Put(tenantID, value)
	}
	status.Touch(tenantID, spec.Code, spec.Title, value.Enabled)
	audit.Note(g, "", "JOB", spec.Code, spec.Title, "保存任务 "+spec.Title)
	httpx.WriteOK(g, c.archiveFrom(value, version, true, spec, tenantID, raw))
}

// Delete 删除控制台新建的任务。内置任务保留。
func (c *JobController) Delete(g *gin.Context) {
	var req models.RunJobRequest
	if !request.Bind(g, &req) {
		return
	}
	spec, ok := catalog.ByCode(req.Code)
	if !ok {
		httpx.WriteErr(g, http.StatusBadRequest, "未知的任务编码")
		return
	}
	if spec.Source != catalog.SourceExternal {
		httpx.WriteErr(g, http.StatusBadRequest, "内置任务不能删除")
		return
	}
	p := middleware.Principal(g)
	if err := c.settings.Delete(g.Request.Context(), policy.Group(spec.Code)); err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "删除任务失败")
		return
	}
	policy.Forget(tenantOf(spec, p.TenantID), spec.Code)
	left, err := c.settings.ActiveCount(g.Request.Context(), policy.Group(spec.Code))
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "删除任务失败")
		return
	}
	if left == 0 {
		if err := registry.Unregister(spec.Code); err != nil {
			httpx.WriteErr(g, http.StatusBadRequest, err.Error())
			return
		}
	}
	audit.Note(g, "", "JOB", spec.Code, spec.Title, "删除任务 "+spec.Title)
	httpx.WriteOK(g, gin.H{"code": spec.Code})
}

func policyFrom(req models.SaveJobRequest) archive.Policy {
	value := archive.Policy{
		Enabled:         true,
		Mode:            req.Mode,
		IntervalMinutes: req.IntervalMinutes,
		RunAt:           req.RunAt,
		Cron:            req.Cron,
		TimeoutSeconds:  req.TimeoutSeconds,
		RetryCount:      req.RetryCount,
		Params: archive.Params{
			AuditLogDays: req.AuditLogDays,
			OpsLogDays:   req.OpsLogDays,
			AlertDays:    req.AlertDays,
			SampleDays:   req.SampleDays,
		},
	}
	if req.Enabled != nil {
		value.Enabled = *req.Enabled
	}
	return value
}

func validateJob(spec catalog.Spec, kind string, value archive.Policy) error {
	useKind := spec.Kind
	if useKind == "" {
		useKind = kind
	}
	if useKind == archive.Kind {
		return archive.Validate(value)
	}
	return policy.ValidateSchedule(policy.Schedule{
		Enabled:         value.Enabled,
		Mode:            value.Mode,
		IntervalMinutes: value.IntervalMinutes,
		RunAt:           value.RunAt,
		Cron:            value.Cron,
		TimeoutSeconds:  value.TimeoutSeconds,
		RetryCount:      value.RetryCount,
	})
}

func encodeJob(spec catalog.Spec, value archive.Policy, paramsText string) (json.RawMessage, error) {
	params, err := policy.ParseObject(paramsText)
	if err != nil {
		return nil, err
	}
	if spec.Kind == archive.Kind {
		if strings.TrimSpace(paramsText) != "" {
			value.Params = archive.ParseParams(params)
		}
		if err := archive.Validate(value); err != nil {
			return nil, err
		}
		return archive.Encode(value)
	}
	if err := policy.ValidateSchedule(policy.Schedule{
		Enabled:         value.Enabled,
		Mode:            value.Mode,
		IntervalMinutes: value.IntervalMinutes,
		RunAt:           value.RunAt,
		Cron:            value.Cron,
		TimeoutSeconds:  value.TimeoutSeconds,
		RetryCount:      value.RetryCount,
	}); err != nil {
		return nil, err
	}
	return policy.EncodeDocument(policy.Document{
		Title:       value.Title,
		Description: value.Description,
		Kind:        value.Kind,
		Source:      value.Source,
		Scope:       value.Scope,
		Schedule: policy.Schedule{
			Enabled:         value.Enabled,
			Mode:            value.Mode,
			IntervalMinutes: value.IntervalMinutes,
			RunAt:           value.RunAt,
			Cron:            value.Cron,
			TimeoutSeconds:  value.TimeoutSeconds,
			RetryCount:      value.RetryCount,
		},
		Params: params,
	})
}

func kindDescription(kind string) string {
	for _, item := range registry.Kinds() {
		if item.Kind == kind {
			return item.Description
		}
	}
	return ""
}

func writeSaveErr(g *gin.Context, err error) {
	if errors.Is(err, dao.ErrConflict) {
		httpx.WriteErr(g, http.StatusConflict, err.Error())
		return
	}
	httpx.WriteErr(g, http.StatusInternalServerError, "保存任务失败")
}
