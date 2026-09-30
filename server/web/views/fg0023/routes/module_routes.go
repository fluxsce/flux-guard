package fg0023routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0023/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0023"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载资源目录。queryResources 是带条件的列表，getResource 按主键取一行。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewResourceController(db)
	router.GET(APIPrefix+"/queryResources", middleware.RequireButton("fg0023:search", ctrl.QueryResources))
	router.GET(APIPrefix+"/getResource", middleware.RequireButton("fg0023:view", ctrl.GetResource))
}
