// Package shellroutes 登记终端会话（经代理出站隧道，不进模块号）。
package shellroutes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/agenttunnel"
	"fluxguard/server/shell"
	"fluxguard/server/web/routes"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	routes.RegisterEntry(Init)
}

// Init 把终端接到会话隧道。控制台仍走 fg0002；Reap 由 cmd/server 启动。
func Init(router *gin.Engine, db *dbx.DB) {
	if db == nil {
		return
	}
	shell.InitGlobal(db, agenttunnel.Global)
}
