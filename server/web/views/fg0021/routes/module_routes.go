package fg0021routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0021/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0021"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载用户查询、维护和角色授权。每个动作使用自己的按钮编码，不能互相代替。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewUserController(db)
	router.GET(APIPrefix+"/queryUsers", middleware.RequireButton("fg0021:search", ctrl.QueryUsers))
	router.GET(APIPrefix+"/queryUserRoles", middleware.RequireButton("fg0021:grant", ctrl.QueryUserRoles))
	router.POST(APIPrefix+"/createUser", middleware.RequireButton("fg0021:add", ctrl.CreateUser))
	router.POST(APIPrefix+"/updateUser", middleware.RequireButton("fg0021:edit", ctrl.UpdateUser))
	router.POST(APIPrefix+"/deleteUser", middleware.RequireButton("fg0021:delete", ctrl.DeleteUser))
	router.POST(APIPrefix+"/saveUserRoles", middleware.RequireButton("fg0021:grant", ctrl.SaveUserRoles))
}
