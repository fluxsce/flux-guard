package fg0041routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
	"fluxguard/server/config"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/fg0041/controllers"
	"fluxguard/server/web/views/fg0041/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	ModuleName = "fg0041"
	APIPrefix  = routes.ModuleAPIPrefix(ModuleName)
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载预警模板。菜单由本模块 dao 补齐。
func Init(router *gin.Engine, db *dbx.DB) {
	dao.NewMenuDAO(db).Ensure(config.TenantID())
	ctrl := controllers.NewTemplateController(alert.Open(db))
	router.GET(APIPrefix+"/queryTemplates", middleware.RequireButton("fg0041:view", ctrl.QueryTemplates))
	router.POST(APIPrefix+"/createTemplate", middleware.RequireButton("fg0041:edit", ctrl.CreateTemplate))
	router.POST(APIPrefix+"/updateTemplate", middleware.RequireButton("fg0041:edit", ctrl.UpdateTemplate))
	router.POST(APIPrefix+"/deleteTemplate", middleware.RequireButton("fg0041:edit", ctrl.DeleteTemplate))
	router.POST(APIPrefix+"/previewTemplate", middleware.RequireButton("fg0041:view", ctrl.PreviewTemplate))
}
