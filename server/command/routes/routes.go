// Package commandroutes 登记代理取令和回执。路径在 protocol，不进模块号。
package commandroutes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/helper/agent"
	"fluxguard/server/command/service"
	"fluxguard/server/web/routes"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	routes.RegisterEntry(Init)
}

// Init 挂上命令通道。令牌校验在命令处理里，不走控制台会话。
func Init(router *gin.Engine, db *dbx.DB) {
	svc := service.Current()
	if db != nil {
		if opened, err := service.Open(db); err == nil {
			svc = opened
		}
	}
	Register(router, agent.Open(db), svc)
}
