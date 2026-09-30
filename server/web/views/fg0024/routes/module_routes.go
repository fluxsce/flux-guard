package fg0024routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0024/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0024"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载环境设置。登录由入口的全局会话校验，这里只校验按钮。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewSettingController(db)
	router.GET(APIPrefix+"/querySettings", middleware.RequireButton("fg0024:view", ctrl.Query))
	router.GET(APIPrefix+"/queryIngestToken", middleware.RequireButton("fg0024:view", ctrl.Reveal))
	router.POST(APIPrefix+"/saveSettings", middleware.RequireButton("fg0024:edit", ctrl.Save))
}
