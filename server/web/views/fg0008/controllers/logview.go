package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/logview"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
)

// OpenLogView 打开一路应用日志跟读（经代理会话隧道）。
func (c *AppController) OpenLogView(g *gin.Context) {
	p := middleware.Principal(g)
	var body struct {
		InstID string `json:"instId"`
	}
	if !request.Bind(g, &body) {
		return
	}
	store := logview.GlobalStore()
	if store == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "跟读未启用")
		return
	}
	view, err := store.Open(g.Request.Context(), p.TenantID, p.UserID, body.InstID)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "APP", view.InstID, view.FileName, "打开日志跟读 "+view.FileName)
	httpx.WriteOK(g, logview.PublicView(view))
}

// LogOutput 拉取跟读增量。
func (c *AppController) LogOutput(g *gin.Context) {
	p := middleware.Principal(g)
	store := logview.GlobalStore()
	if store == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "跟读未启用")
		return
	}
	id := strings.TrimSpace(request.Param(g, "logViewId"))
	offset, _ := strconv.Atoi(strings.TrimSpace(request.Param(g, "offset")))
	chunk, err := store.ReadOutput(p.TenantID, p.UserID, id, offset)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, chunk)
}

// CloseLogView 关闭跟读。
func (c *AppController) CloseLogView(g *gin.Context) {
	p := middleware.Principal(g)
	var body struct {
		LogViewID string `json:"logViewId"`
	}
	if !request.Bind(g, &body) {
		return
	}
	store := logview.GlobalStore()
	if store == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "跟读未启用")
		return
	}
	if err := store.Close(p.TenantID, p.UserID, body.LogViewID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "APP", body.LogViewID, body.LogViewID, "关闭日志跟读")
	httpx.WriteOK(g, map[string]string{"logViewId": body.LogViewID})
}

// TouchLogView 保活，避免空闲回收。
func (c *AppController) TouchLogView(g *gin.Context) {
	p := middleware.Principal(g)
	var body struct {
		LogViewID string `json:"logViewId"`
	}
	if !request.Bind(g, &body) {
		return
	}
	store := logview.GlobalStore()
	if store == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "跟读未启用")
		return
	}
	if err := store.Touch(body.LogViewID, p.UserID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, map[string]string{"logViewId": body.LogViewID})
}
