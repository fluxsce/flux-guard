package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/service"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

// ComponentController 组件目录、连接和曲线。
type ComponentController struct {
	DB *dbx.DB
}

// NewComponentController 创建控制器。
func NewComponentController(db *dbx.DB) *ComponentController {
	return &ComponentController{DB: db}
}

func (c *ComponentController) ready(g *gin.Context) bool {
	if c == nil || c.DB == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "数据库未打开")
		return false
	}
	return true
}

// QueryCatalog 返回组件种类和指标名。种类和页签来自库表。
func (c *ComponentController) QueryCatalog(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	items, err := service.Catalog(g.Request.Context(), c.DB)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, items)
}

// QueryConns 分页列出连接。
func (c *ComponentController) QueryConns(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	kind := request.Param(g, "kindCode")
	if !c.allowKind(g, kind) {
		return
	}
	page := request.PageOf(g)
	rows, total, err := service.List(g.Request.Context(), c.DB, conn.Filter{
		KindCode:      kind,
		ComponentName: request.Param(g, "componentName"),
		StatusFlag:    request.Param(g, "statusFlag"),
		UpFlag:        request.Param(g, "upFlag"),
	}, page)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	request.WritePage(g, rows, total, page)
}

// CreateConn 新增连接。
func (c *ComponentController) CreateConn(g *gin.Context) {
	c.save(g, true)
}

// UpdateConn 修改连接。口令留空表示不改。
func (c *ComponentController) UpdateConn(g *gin.Context) {
	c.save(g, false)
}

func (c *ComponentController) save(g *gin.Context, creating bool) {
	if !c.ready(g) {
		return
	}
	var body models.Input
	if !request.Bind(g, &body) {
		return
	}
	if !c.allowKind(g, body.KindCode) {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentName, "")
	saved, err := service.Save(g.Request.Context(), c.DB, body, creating)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	verb := "修改组件连接 "
	if creating {
		verb = "新增组件连接 "
	}
	detail := verb + audit.Named(saved.ComponentName, saved.ComponentID)
	if saved.KindCode != "" {
		detail += "，种类 " + saved.KindCode
	}
	if strings.TrimSpace(body.Secret) != "" {
		detail += "，并更新了口令"
	}
	audit.Note(g, "", "COMPONENT", saved.ComponentID, saved.ComponentName, detail)
	httpx.WriteOK(g, saved)
}

// DeleteConn 停用连接。
func (c *ComponentController) DeleteConn(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body models.IDInput
	if !request.Bind(g, &body) {
		return
	}
	item, ok := c.owned(g, body.ComponentID)
	if !ok {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, item.ComponentName, "")
	if err := service.Delete(g.Request.Context(), c.DB, body.ComponentID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentID, "删除组件连接 "+body.ComponentID)
	httpx.WriteOK(g, map[string]any{"deleted": true})
}

// TestConn 按弹窗填写试连，不落库。
func (c *ComponentController) TestConn(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body models.Input
	if !request.Bind(g, &body) {
		return
	}
	if !c.allowKind(g, body.KindCode) {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentName, "")
	trial, err := service.Test(g.Request.Context(), c.DB, body)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	detail := "测试组件连接 " + audit.Named(body.ComponentName, body.Address)
	if trial.UpFlag == "Y" {
		detail += "，已连通"
	} else {
		detail += "，未连通"
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentName, detail)
	httpx.WriteOK(g, trial)
}

// ProbeConn 立刻采集一次。
func (c *ComponentController) ProbeConn(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body models.IDInput
	if !request.Bind(g, &body) {
		return
	}
	if _, ok := c.owned(g, body.ComponentID); !ok {
		return
	}
	audit.Note(g, "", "COMPONENT", body.ComponentID, body.ComponentID, "")
	item, err := service.Probe(g.Request.Context(), c.DB, body.ComponentID)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	detail := "采集组件连接 " + audit.Named(item.ComponentName, item.ComponentID)
	if item.UpFlag != "" {
		detail += "，连通 " + item.UpFlag
	}
	audit.Note(g, "", "COMPONENT", item.ComponentID, item.ComponentName, detail)
	httpx.WriteOK(g, item)
}

// QueryLatest 返回最近一个点。
func (c *ComponentController) QueryLatest(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	if _, ok := c.owned(g, request.Param(g, "componentId")); !ok {
		return
	}
	item, err := service.Latest(g.Request.Context(), c.DB, request.Param(g, "componentId"))
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, item)
}

// QuerySeries 返回曲线。
func (c *ComponentController) QuerySeries(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	componentID := request.Param(g, "componentId")
	if _, ok := c.owned(g, componentID); !ok {
		return
	}
	points, err := service.Series(g.Request.Context(), c.DB, componentID, request.Param(g, "limit"), request.Param(g, "from"), request.Param(g, "to"))
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, points)
}

func (c *ComponentController) owned(g *gin.Context, componentID string) (models.Conn, bool) {
	item, err := service.Lookup(g.Request.Context(), c.DB, componentID)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return models.Conn{}, false
	}
	if !c.allowKind(g, item.KindCode) {
		return models.Conn{}, false
	}
	return item, true
}

func (c *ComponentController) allowKind(g *gin.Context, kind string) bool {
	kind = strings.TrimSpace(kind)
	if kind == "" || !catalog.Known(kind) {
		httpx.WriteFail(g, http.StatusBadRequest, fmt.Errorf("请选择组件种类"))
		return false
	}
	principal := middleware.Principal(g)
	if principal.TenantAdmin {
		return true
	}
	access := middleware.Access()
	if access == nil || !access.AllowButton(principal.TenantID, principal.UserID, "fg0007:"+kind) {
		httpx.WriteErr(g, http.StatusForbidden, "没有权限")
		return false
	}
	return true
}
