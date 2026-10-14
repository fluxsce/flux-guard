// Package command 维护命令队列：先拨代理，拨不通再留给长轮询。
// 控制台走 fg0002，代理的取令和回执在本包注册，不进上报入口。
// 实现按功能分在 models、service、routes 子包；本文件是对外门面。
package command

import (
	"fluxguard/server/helper/agent"
	"fluxguard/server/command/models"
	commandroutes "fluxguard/server/command/routes"
	cmdsvc "fluxguard/server/command/service"
	dbx "github.com/fluxsce/dbx/db"
	"github.com/gin-gonic/gin"
)

// Service 读写 FG_COMMAND，并在能拨通时代理执行。
type Service = cmdsvc.Service

// DispatchInput 是控制台提交的一条命令。
type DispatchInput = models.DispatchInput

// View 是控制台看到的一行命令。
type View = models.View

// Open 打开命令服务。
func Open(db *dbx.DB) (*Service, error) {
	return cmdsvc.Open(db)
}

// Current 返回本进程命令服务。尚未 Open 时为 nil。
func Current() *Service {
	return cmdsvc.Current()
}

// Register 挂上代理取令和回执。控制台下发不在这里。
func Register(router *gin.Engine, agents *agent.Store, svc *Service) {
	commandroutes.Register(router, agents, svc)
}
