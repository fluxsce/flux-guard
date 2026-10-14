// Package shellroutes 登记代理终端通道。路径在 protocol，不进模块号。
package shellroutes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/helper/agent"
	"fluxguard/server/shell"
	"fluxguard/server/web/routes"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	routes.RegisterEntry(Init)
}

// Init 挂上代理拉取终端、取输入、回写输出。控制台仍走 fg0002。
func Init(router *gin.Engine, db *dbx.DB) {
	if db == nil {
		return
	}
	shell.RegisterAgent(router, agent.Open(db), shell.OpenStore(db))
}
