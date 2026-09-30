package fg0002routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0002/controllers"

	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0002"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载代理查询、登记和下发。按钮码与页面工具条一致。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewAgentController(db)
	router.GET(APIPrefix+"/queryAgents", middleware.RequireButton("fg0002:search", ctrl.QueryAgents))
	router.POST(APIPrefix+"/registerAgent", middleware.RequireButton("fg0002:edit", ctrl.RegisterAgent))
	router.POST(APIPrefix+"/dispatchCommand", middleware.RequireButton("fg0002:command", ctrl.DispatchCommand))
	router.GET(APIPrefix+"/queryCommand", middleware.RequireButton("fg0002:command", ctrl.QueryCommand))
	router.GET(APIPrefix+"/queryCommands", middleware.RequireButton("fg0002:view", ctrl.QueryCommands))
	router.GET(APIPrefix+"/queryShellAudits", middleware.RequireButton("fg0002:view", ctrl.QueryShellAudits))
	router.POST(APIPrefix+"/openShell", middleware.RequireButton("fg0002:shell", ctrl.OpenShell))
	router.GET(APIPrefix+"/queryShells", middleware.RequireButton("fg0002:shell", ctrl.QueryShells))
	router.POST(APIPrefix+"/closeShell", middleware.RequireButton("fg0002:shell", ctrl.CloseShell))
	router.POST(APIPrefix+"/shellInput", middleware.RequireButton("fg0002:shell", ctrl.ShellInput))
	router.POST(APIPrefix+"/shellResize", middleware.RequireButton("fg0002:shell", ctrl.ShellResize))
	router.GET(APIPrefix+"/shellOutput", middleware.RequireButton("fg0002:shell", ctrl.ShellOutput))
}
