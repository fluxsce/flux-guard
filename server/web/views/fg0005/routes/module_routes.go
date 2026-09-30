package fg0005routes

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
	"fluxguard/server/limits"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0005/controllers"
	"fluxguard/server/web/views/fg0005/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0005"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载主机当前占用和主机预警规则。指标判断留在本模块，算完后把事件投递给预警服务。
func Init(router *gin.Engine, db *dbx.DB) {
	routes.RegisterLoop(func(ctx context.Context) {
		svc := alert.Current()
		if svc == nil || db == nil {
			return
		}
		run := func() { dao.Evaluate(ctx, db, svc) }
		run()
		ticker := time.NewTicker(limits.AlertScanEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	})
	ctrl := controllers.NewHostController(db)
	router.GET(APIPrefix+"/queryHosts", middleware.RequireButton("fg0005:search", ctrl.QueryHosts))
	router.GET(APIPrefix+"/queryHostDetail", middleware.RequireButton("fg0005:view", ctrl.QueryHostDetail))
	router.GET(APIPrefix+"/queryHostSeries", middleware.RequireButton("fg0005:view", ctrl.QueryHostSeries))
	router.GET(APIPrefix+"/queryRules", middleware.RequireButton("fg0005:view", ctrl.QueryRules))
	router.POST(APIPrefix+"/createRule", middleware.RequireButton("fg0005:edit", ctrl.CreateRule))
	router.POST(APIPrefix+"/updateRule", middleware.RequireButton("fg0005:edit", ctrl.UpdateRule))
	router.POST(APIPrefix+"/deleteRule", middleware.RequireButton("fg0005:edit", ctrl.DeleteRule))
}
