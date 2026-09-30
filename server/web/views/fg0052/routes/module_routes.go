package fg0052routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/config"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0052/controllers"
	"fluxguard/server/web/views/fg0052/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0052"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载代理操作日志查询。写操作不在这一页。
func Init(router *gin.Engine, db *dbx.DB) {
	dao.NewMenuDAO(db).Ensure(config.TenantID())
	ctrl := controllers.NewLogController(db)
	router.GET(APIPrefix+"/queryLogs", middleware.RequireButton("fg0052:search", ctrl.QueryLogs))
	router.GET(APIPrefix+"/getLog", middleware.RequireButton("fg0052:view", ctrl.GetLog))
}
