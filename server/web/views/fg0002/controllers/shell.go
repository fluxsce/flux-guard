package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
)

// OpenShell 为当前用户再开一个终端。
func (c *AgentController) OpenShell(g *gin.Context) {
	var body struct {
		AgentID string `json:"agentId"`
	}
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "AGENT", body.AgentID, body.AgentID, "打开终端")
	p := middleware.Principal(g)
	view, err := c.shells.Open(g.Request.Context(), p.TenantID, p.UserID, displayName(p), body.AgentID)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	audit.Note(g, "", "AGENT", view.AgentID, view.AgentID, "打开终端")
	httpx.WriteOK(g, view)
}

// QueryShells 列出这台代理上仍打开的终端，包含其他人的。
func (c *AgentController) QueryShells(g *gin.Context) {
	p := middleware.Principal(g)
	if err := c.shells.Touch(g.Request.Context(), p.TenantID, p.UserID, request.Param(g, "agentId")); err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	list, err := c.shells.List(g.Request.Context(), p.TenantID, request.Param(g, "agentId"))
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, list)
}

// CloseShell 关闭一个终端。
func (c *AgentController) CloseShell(g *gin.Context) {
	var body struct {
		ShellID string `json:"shellId"`
	}
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "AGENT", body.ShellID, body.ShellID, "关闭终端")
	p := middleware.Principal(g)
	err := c.shells.Close(g.Request.Context(), p.TenantID, p.UserID, body.ShellID, p.TenantAdmin)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	audit.Note(g, "", "AGENT", body.ShellID, body.ShellID, "关闭终端")
	httpx.WriteOK(g, map[string]any{"shellId": body.ShellID})
}

// ShellInput 把一行交给打开人自己的终端。
func (c *AgentController) ShellInput(g *gin.Context) {
	var body struct {
		ShellID string `json:"shellId"`
		Text    string `json:"text"`
	}
	if !request.Bind(g, &body) {
		return
	}
	p := middleware.Principal(g)
	err := c.shells.WriteInput(g.Request.Context(), p.TenantID, p.UserID, body.ShellID, body.Text)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, map[string]string{"shellId": body.ShellID})
}

// ShellResize 把页面上的行列交给这一路终端。
func (c *AgentController) ShellResize(g *gin.Context) {
	var body struct {
		ShellID string `json:"shellId"`
		Cols    int    `json:"cols"`
		Rows    int    `json:"rows"`
	}
	if !request.Bind(g, &body) {
		return
	}
	p := middleware.Principal(g)
	err := c.shells.WriteSize(g.Request.Context(), p.TenantID, p.UserID, body.ShellID, body.Cols, body.Rows)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, map[string]any{"shellId": body.ShellID, "cols": body.Cols, "rows": body.Rows})
}

// ShellOutput 返回打开人尚未读过的输出。
func (c *AgentController) ShellOutput(g *gin.Context) {
	offset, _ := strconv.Atoi(request.Param(g, "offset"))
	if offset < 0 {
		offset = 0
	}
	p := middleware.Principal(g)
	chunk, err := c.shells.ReadOutput(g.Request.Context(), p.TenantID, p.UserID, request.Param(g, "shellId"), offset)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	httpx.WriteOK(g, chunk)
}

// QueryShellAudits 按代理列出终端会话账本（开闭与回车命令行）。
func (c *AgentController) QueryShellAudits(g *gin.Context) {
	p := middleware.Principal(g)
	list, err := c.shells.ListAudits(g.Request.Context(), p.TenantID, request.Param(g, "agentId"), 100)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, list)
}

func displayName(p request.Principal) string {
	if p.RealName != "" {
		return p.RealName
	}
	return p.UserName
}
