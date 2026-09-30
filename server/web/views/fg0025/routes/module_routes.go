package fg0025routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0025/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0025"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载系统任务调度。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewJobController(db)
	router.GET(APIPrefix+"/queryJobs", middleware.RequireButton("fg0025:view", ctrl.QueryJobs))
	router.GET(APIPrefix+"/getJob", middleware.RequireButton("fg0025:view", ctrl.GetJob))
	router.GET(APIPrefix+"/queryJobLogs", middleware.RequireButton("fg0025:view", ctrl.QueryLogs))
	router.POST(APIPrefix+"/saveArchiveJob", middleware.RequireButton("fg0025:edit", ctrl.SaveArchive))
	router.POST(APIPrefix+"/saveJob", middleware.RequireButton("fg0025:edit", ctrl.Save))
	router.POST(APIPrefix+"/deleteJob", middleware.RequireButton("fg0025:edit", ctrl.Delete))
	router.POST(APIPrefix+"/runJob", middleware.RequireButton("fg0025:edit", ctrl.Run))
}
