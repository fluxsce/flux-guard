// Package loginroutes 登记登录模块。前缀固定为 /fluxguard/api/auth，与 gateway hub0001 单独指定前缀的方式相同。
package loginroutes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/server/web/routes"
	"fluxguard/server/web/views/login/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	// ModuleName 模块目录名。
	ModuleName = "login"
	// APIPrefix 本模块接口前缀。控制台沿用 /auth。
	APIPrefix = routes.APIRoot + "/auth"
)

func init() {
	routes.RegisterModuleRoutes(ModuleName, Init)
}

// Init 挂载登录模块。登录和验证码公开，其余接口必须已登录。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewAuthController(db)
	authGroup := router.Group(APIPrefix)

	authGroup.POST("/login", routes.PublicAPI(), ctrl.Login)
	authGroup.POST("/captcha", routes.PublicAPI(), ctrl.Captcha)

	sessionGroup := authGroup.Group("")
	sessionGroup.Use(routes.PermissionRequired()...)
	{
		sessionGroup.POST("/logout", ctrl.Logout)
		sessionGroup.GET("/me", ctrl.Me)
		sessionGroup.GET("/profile", ctrl.Profile)
		sessionGroup.POST("/profile", ctrl.SaveProfile)
		sessionGroup.GET("/menus", ctrl.Menus)
	}
}
