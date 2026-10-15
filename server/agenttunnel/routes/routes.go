// Package agenttunnelroutes 登记代理会话隧道入口。
package agenttunnelroutes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/agenttunnel"
	"fluxguard/server/helper/agent"
	"fluxguard/server/web/routes"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	routes.RegisterEntry(Init)
}

// Init 挂上隧道。
func Init(router *gin.Engine, db *dbx.DB) {
	if db == nil {
		return
	}
	agenttunnel.Register(router, agent.Open(db), agenttunnel.Global)
}
