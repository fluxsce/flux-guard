package controllers

import (
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"github.com/gin-gonic/gin"

	"fluxguard/server/agent"
	"fluxguard/server/plugin"
	"fluxguard/server/web/ingest/service"
	dbx "github.com/fluxsce/dbx/db"
)

// OverviewController 总览。
type OverviewController struct {
	agents *agent.Store
	Ingest *service.Service
}

// NewOverviewController 创建总览控制器。
func NewOverviewController(db *dbx.DB) *OverviewController {
	return &OverviewController{agents: agent.Open(db), Ingest: service.Open(db)}
}

// Overview 返回代理、服务器扩展和事件规模。
func (c *OverviewController) Overview(g *gin.Context) {
	p := middleware.Principal(g)
	agents := c.agents.ListAgents(p.TenantID)
	online := 0
	for _, item := range agents {
		if item.OnlineFlag == "Y" {
			online++
		}
	}
	stats := c.Ingest.Stats(p.TenantID)
	httpx.WriteOK(g, map[string]any{
		"agentCount":       len(agents),
		"onlineCount":      online,
		"pluginCount":      len(plugin.Infos()),
		"eventCount":       stats.EventCount,
		"lastReceivedTime": stats.LastReceivedTime,
	})
}
