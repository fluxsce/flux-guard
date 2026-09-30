package fg0042routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0042/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0042"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载通知渠道。渠道关联模板，正文在模板上维护。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewChannelController(alert.Open(db))
	router.GET(APIPrefix+"/queryChannels", middleware.RequireButton("fg0042:view", ctrl.QueryChannels))
	router.POST(APIPrefix+"/createChannel", middleware.RequireButton("fg0042:edit", ctrl.CreateChannel))
	router.POST(APIPrefix+"/updateChannel", middleware.RequireButton("fg0042:edit", ctrl.UpdateChannel))
	router.POST(APIPrefix+"/deleteChannel", middleware.RequireButton("fg0042:edit", ctrl.DeleteChannel))
	router.POST(APIPrefix+"/testChannel", middleware.RequireButton("fg0042:edit", ctrl.TestChannel))
}
