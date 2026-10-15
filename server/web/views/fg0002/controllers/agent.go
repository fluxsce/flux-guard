package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"strings"

	"fluxguard/server/helper/agent"
	"fluxguard/server/command"
	"fluxguard/server/shell"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

// AgentController 采集代理管理。
type AgentController struct {
	db       *dbx.DB
	agents   *agent.Store
	shells   *shell.Store
	Commands *command.Service
}

// NewAgentController 创建代理控制器。
func NewAgentController(db *dbx.DB) *AgentController {
	shells := shell.Global()
	if shells == nil {
		shells = shell.OpenStore(db)
	}
	return &AgentController{db: db, agents: agent.Open(db), shells: shells, Commands: command.Current()}
}

// QueryAgents 列出本租户代理。
func (c *AgentController) QueryAgents(g *gin.Context) {
	p := middleware.Principal(g)
	httpx.WriteOK(g, c.agents.ListAgents(p.TenantID))
}

// RegisterAgent 登记代理并返回一次明文令牌。
func (c *AgentController) RegisterAgent(g *gin.Context) {
	var body agent.RegisterInput
	if !request.Bind(g, &body) {
		return
	}
	name := strings.TrimSpace(body.AgentName)
	audit.Note(g, "", "AGENT", "", name, "登记采集代理 "+name)
	view, token, err := c.agents.RegisterAgent(g.Request.Context(), body)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	audit.Note(g, "", "AGENT", view.AgentID, view.AgentName, "登记采集代理 "+audit.Named(view.AgentName, view.HostName))
	httpx.WriteOK(g, map[string]any{"agent": view, "agentToken": token})
}

// deleteAgentsBody 批量删除代理的请求体。
type deleteAgentsBody struct {
	AgentIDs []string `json:"agentIds"`
}

// DeleteAgents 物理删除选中的采集代理。
func (c *AgentController) DeleteAgents(g *gin.Context) {
	var body deleteAgentsBody
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "AGENT", "", "", "删除采集代理")
	n, err := c.agents.DeleteAgents(g.Request.Context(), body.AgentIDs)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	audit.Note(g, "", "AGENT", "", "", "删除采集代理 "+strings.TrimSpace(strings.Join(body.AgentIDs, ",")))
	httpx.WriteOK(g, map[string]any{"deleted": n})
}

// renameAgentBody 改显示名与备注的请求体。
type renameAgentBody struct {
	AgentID   string `json:"agentId"`
	AgentName string `json:"agentName"`
	NoteText  string `json:"noteText"`
}

// RenameAgent 改显示名与备注。上报不会冲掉这里改过的名字。
func (c *AgentController) RenameAgent(g *gin.Context) {
	var body renameAgentBody
	if !request.Bind(g, &body) {
		return
	}
	name := strings.TrimSpace(body.AgentName)
	audit.Note(g, "", "AGENT", body.AgentID, name, "修改代理显示名 "+name)
	view, err := c.agents.RenameAgent(g.Request.Context(), body.AgentID, body.AgentName, body.NoteText)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	audit.Note(g, "", "AGENT", view.AgentID, view.AgentName, "修改代理显示名 "+view.AgentName)
	httpx.WriteOK(g, view)
}
