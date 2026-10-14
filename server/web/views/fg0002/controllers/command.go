package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"

	"github.com/gin-gonic/gin"

	"fluxguard/protocol"
	"fluxguard/server/command"
)

// DispatchCommand 对一台主机代理下发名单内动作，并返回这一行的当前状态。
func (c *AgentController) DispatchCommand(g *gin.Context) {
	if c.Commands == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "命令通道未启用")
		return
	}
	p := middleware.Principal(g)
	var body command.DispatchInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "AGENT", body.AgentID, body.AgentID, commandText(body))
	view, err := c.Commands.Dispatch(g.Request.Context(), p.TenantID, p.UserID, body)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	audit.Note(g, "", "AGENT", view.AgentID, view.AgentID, commandText(body))
	httpx.WriteOK(g, view)
}

func commandText(in command.DispatchInput) string {
	switch in.Action {
	case protocol.ActionCollectNow:
		return "让代理立即采集"
	case protocol.ActionIntervalSet:
		return fmt.Sprintf("把上报间隔改为 %d 秒", in.IntervalSec)
	case protocol.ActionAgentUpdate:
		return "升级代理到 " + strings.TrimSpace(in.Version)
	default:
		action := strings.TrimSpace(in.Action)
		if action == "" {
			return "下发命令"
		}
		return "下发命令 " + action
	}
}

// QueryCommand 读取一条命令的状态。拨不通时页面继续看同一行。
func (c *AgentController) QueryCommand(g *gin.Context) {
	if c.Commands == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "命令通道未启用")
		return
	}
	p := middleware.Principal(g)
	view, err := c.Commands.Get(g.Request.Context(), p.TenantID, request.Param(g, "commandId"))
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, view)
}

// QueryCommands 按代理列出最近的命令执行记录（含回执）。
func (c *AgentController) QueryCommands(g *gin.Context) {
	if c.Commands == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "命令通道未启用")
		return
	}
	p := middleware.Principal(g)
	list, err := c.Commands.ListByAgent(g.Request.Context(), p.TenantID, request.Param(g, "agentId"), 50)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, list)
}
