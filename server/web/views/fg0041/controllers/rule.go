package controllers

import (
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"net/http"

	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
)

// TemplateController 预警模板查询和维护。
type TemplateController struct {
	Alert *alert.Service
}

// NewTemplateController 创建模板控制器。
func NewTemplateController(svc *alert.Service) *TemplateController {
	return &TemplateController{Alert: svc}
}

func (c *TemplateController) ready(g *gin.Context) bool {
	if c == nil || c.Alert == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "预警服务未启用")
		return false
	}
	return true
}

// QueryTemplates 列出本租户模板。
func (c *TemplateController) QueryTemplates(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	page := request.PageOf(g)
	rows, total, err := c.Alert.PageTemplates(middleware.Principal(g).TenantID, alert.TemplateQuery{
		TemplateName: request.Param(g, "templateName"),
		ChannelKind:  request.Param(g, "channelKind"),
		StatusFlag:   request.Param(g, "statusFlag"),
	}, page)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	request.WritePage(g, rows, total, page)
}

// CreateTemplate 新增模板。
func (c *TemplateController) CreateTemplate(g *gin.Context) {
	c.save(g, true)
}

// UpdateTemplate 修改模板。
func (c *TemplateController) UpdateTemplate(g *gin.Context) {
	c.save(g, false)
}

func (c *TemplateController) save(g *gin.Context, creating bool) {
	if !c.ready(g) {
		return
	}
	var body alert.TemplateInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "TEMPLATE", body.TemplateID, body.TemplateName, "")
	saved, err := c.Alert.SaveTemplate(g.Request.Context(), body, creating)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	verb := "修改预警模板 "
	if creating {
		verb = "新增预警模板 "
	}
	detail := verb + audit.Named(saved.TemplateName, saved.TemplateID)
	if saved.ChannelKind != "" {
		detail += "，种类 " + saved.ChannelKind
	}
	audit.Note(g, "", "TEMPLATE", saved.TemplateID, saved.TemplateName, detail)
	httpx.WriteOK(g, saved)
}

// DeleteTemplate 删除自定义模板。
func (c *TemplateController) DeleteTemplate(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body alert.IDInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "TEMPLATE", body.TemplateID, body.TemplateID, "")
	if err := c.Alert.DeleteTemplate(g.Request.Context(), body.TemplateID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "TEMPLATE", body.TemplateID, body.TemplateID, "删除预警模板 "+body.TemplateID)
	httpx.WriteOK(g, map[string]any{"deleted": true})
}

// PreviewTemplate 用样例数据渲染标题和正文，不发送。
func (c *TemplateController) PreviewTemplate(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body alert.PreviewInput
	if !request.Bind(g, &body) {
		return
	}
	row, err := c.Alert.PreviewTemplate(middleware.Principal(g).TenantID, body)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, row)
}
