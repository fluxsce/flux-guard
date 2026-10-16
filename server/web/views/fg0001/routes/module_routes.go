package fg0001routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0001/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0001"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载总览。按钮码与 FG_AUTH_RESOURCE 中的 fg0001:view、fg0001:upgrade 一致。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewOverviewController(db)
	router.GET(APIPrefix+"/overview", middleware.RequireButton("fg0001:view", ctrl.Overview))
	router.POST(APIPrefix+"/upgradeServer", middleware.RequireButton("fg0001:upgrade", ctrl.UpgradeServer))
}
