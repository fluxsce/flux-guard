package fg0022routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0022/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0022"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载角色查询、维护和授权保存。每个动作使用自己的按钮编码，不能互相代替。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewRoleController(db)
	router.GET(APIPrefix+"/queryRoles", middleware.RequireButton("fg0022:search", ctrl.QueryRoles))
	router.GET(APIPrefix+"/getRole", middleware.RequireButton("fg0022:view", ctrl.GetRole))
	router.GET(APIPrefix+"/queryResources", middleware.RequireButton("fg0022:grant", ctrl.QueryResources))
	router.POST(APIPrefix+"/createRole", middleware.RequireButton("fg0022:add", ctrl.CreateRole))
	router.POST(APIPrefix+"/updateRole", middleware.RequireButton("fg0022:edit", ctrl.UpdateRole))
	router.POST(APIPrefix+"/deleteRole", middleware.RequireButton("fg0022:delete", ctrl.DeleteRole))
	router.POST(APIPrefix+"/saveRoleResources", middleware.RequireButton("fg0022:grant", ctrl.SaveRoleResources))
}
