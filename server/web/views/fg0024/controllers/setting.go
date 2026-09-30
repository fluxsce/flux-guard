package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"

	"github.com/gin-gonic/gin"

	"fluxguard/server/setting"
	accessdao "fluxguard/server/web/middleware/dao"
	"fluxguard/server/web/views/fg0024/dao"
	"fluxguard/server/web/views/fg0024/models"
	dbx "github.com/fluxsce/dbx/db"
)

// SettingController 按分组读写租户环境设置。
type SettingController struct {
	access   *accessdao.Access
	settings *dao.SettingDAO
}

// NewSettingController 创建环境设置控制器。
func NewSettingController(db *dbx.DB) *SettingController {
	return &SettingController{access: accessdao.Open(db), settings: dao.NewSettingDAO(db)}
}

// Query 返回访问策略和全局环境变量。路由已校验 fg0024:view，供控制台套用超时和分页。
func (c *SettingController) Query(g *gin.Context) {
	p := middleware.Principal(g)
	timeout, timeoutVersion, err := c.loadTimeout(g, p.TenantID)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载环境设置失败")
		return
	}
	vars, varsVersion, err := c.loadEnvVars(g, p.TenantID)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载环境设置失败")
		return
	}
	token, tokenVersion, err := c.loadIngest(g, p.TenantID)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载环境设置失败")
		return
	}
	editable := c.access.Allow(p.TenantID, p.UserID, "fg0024:edit")
	httpx.WriteOK(g, models.Settings{
		Editable: editable,
		WebTimeout: models.WebTimeout{
			RequestTimeoutSeconds: timeout.RequestTimeoutSeconds,
			SessionExpireHours:    timeout.SessionExpireHours,
			DefaultPageSize:       timeout.DefaultPageSize,
			MaxPageSize:           timeout.MaxPageSize,
			CurrentVersion:        timeoutVersion,
			Editable:              editable,
		},
		EnvVars: models.EnvVarsView{
			Items:          setting.MaskEnvVars(vars),
			CurrentVersion: varsVersion,
		},
		IngestToken: ingestView(token, tokenVersion, ""),
	})
}

// Reveal 解出已签发的上报令牌，供控制台查看和复制后写入代理。
func (c *SettingController) Reveal(g *gin.Context) {
	p := middleware.Principal(g)
	audit.Note(g, audit.ActionUpdate, "SETTING", setting.GroupIngestToken, "上报令牌", "查看上报令牌")
	token, version, err := c.loadIngest(g, p.TenantID)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载环境设置失败")
		return
	}
	plain := setting.RevealIngest(token)
	if token.TokenHash != "" && plain == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "无法读出上报令牌，请轮换后再分发")
		return
	}
	audit.Note(g, audit.ActionUpdate, "SETTING", setting.GroupIngestToken, "上报令牌", "查看上报令牌")
	httpx.WriteOK(g, ingestView(token, version, plain))
}

// Save 按分组保存，并立即进入进程缓存。
func (c *SettingController) Save(g *gin.Context) {
	var req models.SaveRequest
	if !request.Bind(g, &req) {
		return
	}
	p := middleware.Principal(g)
	switch req.GroupCode {
	case "", setting.GroupWebTimeout:
		c.saveTimeout(g, p, req)
	case setting.GroupEnvVars:
		c.saveEnvVars(g, p, req)
	case setting.GroupIngestToken:
		c.saveIngest(g, p, req)
	default:
		audit.Note(g, "", "SETTING", req.GroupCode, req.GroupCode, "保存环境设置 "+req.GroupCode)
		httpx.WriteErr(g, http.StatusBadRequest, "未知的设置分组")
	}
}

func (c *SettingController) loadTimeout(g *gin.Context, tenantID string) (setting.WebTimeout, int, error) {
	content, version, ok, err := c.settings.Get(g.Request.Context(), tenantID, setting.GroupWebTimeout)
	if err != nil {
		return setting.WebTimeout{}, 0, err
	}
	value := setting.Default()
	if ok {
		value = setting.Parse(content)
		setting.Put(tenantID, value)
	}
	return value, version, nil
}

func (c *SettingController) loadEnvVars(g *gin.Context, tenantID string) (setting.EnvVars, int, error) {
	content, version, ok, err := c.settings.Get(g.Request.Context(), tenantID, setting.GroupEnvVars)
	if err != nil {
		return setting.EnvVars{}, 0, err
	}
	value := setting.DefaultEnvVars()
	if ok {
		value = setting.ParseEnvVars(content)
		setting.PutEnvVars(tenantID, value)
	}
	return value, version, nil
}

func (c *SettingController) saveTimeout(g *gin.Context, p request.Principal, req models.SaveRequest) {
	audit.Note(g, "", "SETTING", setting.GroupWebTimeout, "访问策略", "修改访问策略")
	value := setting.WebTimeout{
		RequestTimeoutSeconds: req.RequestTimeoutSeconds,
		SessionExpireHours:    req.SessionExpireHours,
		DefaultPageSize:       req.DefaultPageSize,
		MaxPageSize:           req.MaxPageSize,
	}
	if err := setting.Validate(value); err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	content, err := setting.Encode(value)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	version, err := c.settings.Save(g.Request.Context(), setting.GroupWebTimeout, req.CurrentVersion, content)
	if errors.Is(err, dao.ErrConflict) {
		httpx.WriteErr(g, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	setting.Put(p.TenantID, value)
	audit.Note(g, "", "SETTING", setting.GroupWebTimeout, "访问策略", fmt.Sprintf(
		"修改访问策略：接口超时 %d 秒，会话 %d 小时，默认每页 %d 条，最多 %d 条",
		value.RequestTimeoutSeconds, value.SessionExpireHours, value.DefaultPageSize, value.MaxPageSize,
	))
	httpx.WriteOK(g, models.WebTimeout{
		RequestTimeoutSeconds: value.RequestTimeoutSeconds,
		SessionExpireHours:    value.SessionExpireHours,
		DefaultPageSize:       value.DefaultPageSize,
		MaxPageSize:           value.MaxPageSize,
		CurrentVersion:        version,
		Editable:              true,
	})
}

func (c *SettingController) loadIngest(g *gin.Context, tenantID string) (setting.IngestToken, int, error) {
	content, version, ok, err := c.settings.Get(g.Request.Context(), tenantID, setting.GroupIngestToken)
	if err != nil {
		return setting.IngestToken{}, 0, err
	}
	if !ok {
		return setting.IngestToken{}, 0, nil
	}
	value := setting.ParseIngest(content)
	setting.PutIngest(tenantID, value)
	return value, version, nil
}

func (c *SettingController) saveIngest(g *gin.Context, p request.Principal, req models.SaveRequest) {
	audit.Note(g, "", "SETTING", setting.GroupIngestToken, "上报令牌", ingestDetail(req.Action))
	current, version, err := c.loadIngest(g, p.TenantID)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	if req.CurrentVersion != version {
		httpx.WriteErr(g, http.StatusConflict, dao.ErrConflict.Error())
		return
	}
	plain := ""
	next := current
	switch req.Action {
	case "rotate":
		issued, token := setting.IssueIngestToken()
		next = issued
		plain = token
	case "enable", "disable":
		if next.TokenHash == "" {
			httpx.WriteErr(g, http.StatusBadRequest, "尚未签发出上报令牌")
			return
		}
		if req.Action == "disable" {
			next.StatusFlag = "N"
		} else {
			next.StatusFlag = "Y"
		}
	default:
		httpx.WriteErr(g, http.StatusBadRequest, "未知的令牌操作")
		return
	}
	content, err := setting.EncodeIngest(next)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	saved, err := c.settings.Save(g.Request.Context(), setting.GroupIngestToken, version, content)
	if errors.Is(err, dao.ErrConflict) {
		httpx.WriteErr(g, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	setting.PutIngest(p.TenantID, next)
	audit.Note(g, "", "SETTING", setting.GroupIngestToken, "上报令牌", ingestDetail(req.Action))
	httpx.WriteOK(g, ingestView(next, saved, plain))
}

func ingestDetail(action string) string {
	switch action {
	case "rotate":
		return "轮换上报令牌"
	case "enable":
		return "启用上报令牌"
	case "disable":
		return "停用上报令牌"
	default:
		return "修改上报令牌"
	}
}

func ingestView(item setting.IngestToken, version int, plain string) models.IngestTokenView {
	return models.IngestTokenView{
		Configured:     item.TokenHash != "",
		StatusFlag:     item.StatusFlag,
		Hint:           item.Hint,
		CurrentVersion: version,
		AgentToken:     plain,
	}
}

func (c *SettingController) saveEnvVars(g *gin.Context, p request.Principal, req models.SaveRequest) {
	audit.Note(g, "", "SETTING", setting.GroupEnvVars, "环境变量", envDetail(req.Items))
	prev, _, err := c.loadEnvVars(g, p.TenantID)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	value, err := setting.MergeEnvVars(req.Items, prev)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	content, err := setting.EncodeEnvVars(value)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	version, err := c.settings.Save(g.Request.Context(), setting.GroupEnvVars, req.CurrentVersion, content)
	if errors.Is(err, dao.ErrConflict) {
		httpx.WriteErr(g, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "保存环境设置失败")
		return
	}
	setting.PutEnvVars(p.TenantID, value)
	audit.Note(g, "", "SETTING", setting.GroupEnvVars, "环境变量", envDetail(req.Items))
	httpx.WriteOK(g, models.EnvVarsView{
		Items:          setting.MaskEnvVars(value),
		CurrentVersion: version,
	})
}

func envDetail(items []setting.EnvVarInput) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if name := strings.TrimSpace(item.Name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "修改环境变量"
	}
	return "修改环境变量：" + strings.Join(names, "、")
}
