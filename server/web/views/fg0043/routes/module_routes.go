package fg0043routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
	"fluxguard/server/config"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0043/controllers"
	"fluxguard/server/web/views/fg0043/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0043"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载预警日志。
func Init(router *gin.Engine, db *dbx.DB) {
	dao.NewMenuDAO(db).Ensure(config.TenantID())
	ctrl := controllers.NewEventController(alert.Open(db))
	router.GET(APIPrefix+"/queryLogs", middleware.RequireButton("fg0043:view", ctrl.QueryLogs))
}
