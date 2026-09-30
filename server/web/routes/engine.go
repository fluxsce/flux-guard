package routes

import "github.com/gin-gonic/gin"

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// NewEngine 创建管理端 Gin 引擎，并挂上全局中间件。
func NewEngine() *gin.Engine {
	engine := gin.New()
	ApplyGlobalMiddleware(engine)
	return engine
}
