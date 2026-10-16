package fg0007routes

import (
	"context"

	"github.com/gin-gonic/gin"

	"fluxguard/server/component/sched"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0007/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0007"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载组件连接和采集曲线，并启动直连采集循环。
// 菜单、种类和页签的初始行在 scripts/db，启动时不再补。
func Init(router *gin.Engine, db *dbx.DB) {
	routes.RegisterLoop(func(ctx context.Context) {
		sched.Loop(ctx, db)
	})
	ctrl := controllers.NewComponentController(db)
	router.GET(APIPrefix+"/queryCatalog", middleware.RequireButton("fg0007:view", ctrl.QueryCatalog))
	router.GET(APIPrefix+"/queryConns", middleware.RequireButton("fg0007:search", ctrl.QueryConns))
	router.POST(APIPrefix+"/createConn", middleware.RequireButton("fg0007:edit", ctrl.CreateConn))
	router.POST(APIPrefix+"/updateConn", middleware.RequireButton("fg0007:edit", ctrl.UpdateConn))
	router.POST(APIPrefix+"/deleteConn", middleware.RequireButton("fg0007:edit", ctrl.DeleteConn))
	router.POST(APIPrefix+"/testConn", middleware.RequireButton("fg0007:edit", ctrl.TestConn))
	router.POST(APIPrefix+"/probeConn", middleware.RequireButton("fg0007:edit", ctrl.ProbeConn))
	router.GET(APIPrefix+"/queryLatest", middleware.RequireButton("fg0007:view", ctrl.QueryLatest))
	router.GET(APIPrefix+"/querySeries", middleware.RequireButton("fg0007:view", ctrl.QuerySeries))
	router.POST(APIPrefix+"/execConn", middleware.RequireButton("fg0007:shell", ctrl.ExecConn))
}
