package fg0008routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/logview"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0008/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0008"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载应用管理。清单来自应用侧代理上报，更新命令也发给该代理。
func Init(router *gin.Engine, db *dbx.DB) {
	logview.InitGlobal(db)
	ctrl := controllers.NewAppController(db)
	router.GET(APIPrefix+"/queryApps", middleware.RequireButton("fg0008:search", ctrl.QueryApps))
	router.GET(APIPrefix+"/querySites", middleware.RequireButton("fg0008:search", ctrl.QuerySites))
	router.GET(APIPrefix+"/querySiteFiles", middleware.RequireButton("fg0008:view", ctrl.QuerySiteFiles))
	router.GET(APIPrefix+"/queryInstances", middleware.RequireButton("fg0008:view", ctrl.QueryInstances))
	router.GET(APIPrefix+"/queryReleases", middleware.RequireButton("fg0008:view", ctrl.QueryReleases))
	router.GET(APIPrefix+"/queryRelease", middleware.RequireButton("fg0008:view", ctrl.QueryRelease))
	router.POST(APIPrefix+"/requestFileFetch", middleware.RequireButton("fg0008:view", ctrl.RequestFileFetch))
	router.GET(APIPrefix+"/queryFileFetch", middleware.RequireButton("fg0008:view", ctrl.QueryFileFetch))
	router.GET(APIPrefix+"/downloadFileFetch", middleware.RequireButton("fg0008:view", ctrl.DownloadFileFetch))
	router.POST(APIPrefix+"/openLogView", middleware.RequireButton("fg0008:view", ctrl.OpenLogView))
	router.GET(APIPrefix+"/logOutput", middleware.RequireButton("fg0008:view", ctrl.LogOutput))
	router.POST(APIPrefix+"/closeLogView", middleware.RequireButton("fg0008:view", ctrl.CloseLogView))
	router.POST(APIPrefix+"/touchLogView", middleware.RequireButton("fg0008:view", ctrl.TouchLogView))
	router.POST(APIPrefix+"/rename", middleware.RequireButton("fg0008:edit", ctrl.Rename))
	router.POST(APIPrefix+"/updateSiteNote", middleware.RequireButton("fg0008:edit", ctrl.UpdateSiteNote))
	router.POST(APIPrefix+"/ignore", middleware.RequireButton("fg0008:edit", ctrl.Ignore))
	router.POST(APIPrefix+"/release", middleware.RequireButton("fg0008:release", ctrl.Release))
	router.POST(APIPrefix+"/continueRelease", middleware.RequireButton("fg0008:release", ctrl.ContinueRelease))
	router.POST(APIPrefix+"/rollback", middleware.RequireButton("fg0008:release", ctrl.Rollback))
	router.POST(APIPrefix+"/runtimeControl", middleware.RequireButton("fg0008:release", ctrl.RuntimeControl))
	router.GET(APIPrefix+"/queryCommand", middleware.RequireButton("fg0008:view", ctrl.QueryCommand))
}
