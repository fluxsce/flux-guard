package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"strings"

	"fluxguard/server/agent"
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
	agents   *agent.Store
	shells   *shell.Store
	Commands *command.Service
}

// NewAgentController 创建代理控制器。
func NewAgentController(db *dbx.DB) *AgentController {
	return &AgentController{agents: agent.Open(db), shells: shell.OpenStore(db), Commands: command.Current()}
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
