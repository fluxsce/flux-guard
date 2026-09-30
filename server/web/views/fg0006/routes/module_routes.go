package fg0006routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0006/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0006"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载 Java 应用当前值和历史曲线。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewJavaController(db)
	router.GET(APIPrefix+"/queryApps", middleware.RequireButton("fg0006:search", ctrl.QueryApps))
	router.GET(APIPrefix+"/queryDetail", middleware.RequireButton("fg0006:view", ctrl.QueryDetail))
	router.GET(APIPrefix+"/querySeries", middleware.RequireButton("fg0006:view", ctrl.QuerySeries))
}
