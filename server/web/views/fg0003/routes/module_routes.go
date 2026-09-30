package fg0003routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0003/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0003"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载服务器扩展目录查询。
func Init(router *gin.Engine, _ *dbx.DB) {
	ctrl := controllers.NewPluginController()
	router.GET(APIPrefix+"/queryPlugins", middleware.RequireButton("fg0003:view", ctrl.QueryPlugins))
}
