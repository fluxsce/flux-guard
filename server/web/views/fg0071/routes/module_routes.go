package fg0071routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0071/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0071"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载应用管理。清单来自应用侧代理上报，更新命令也发给该代理。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewAppController(db)
	router.GET(APIPrefix+"/queryApps", middleware.RequireButton("fg0071:search", ctrl.QueryApps))
	router.GET(APIPrefix+"/queryInstances", middleware.RequireButton("fg0071:view", ctrl.QueryInstances))
	router.GET(APIPrefix+"/queryRelease", middleware.RequireButton("fg0071:view", ctrl.QueryRelease))
	router.POST(APIPrefix+"/rename", middleware.RequireButton("fg0071:edit", ctrl.Rename))
	router.POST(APIPrefix+"/ignore", middleware.RequireButton("fg0071:edit", ctrl.Ignore))
	router.POST(APIPrefix+"/release", middleware.RequireButton("fg0071:release", ctrl.Release))
}
