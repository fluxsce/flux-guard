package routes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/common/buildinfo"
	"fluxguard/server/web/httpx"
)

func mountHealth(router *gin.Engine) {
	router.GET("/healthz", func(c *gin.Context) {
		httpx.WriteOK(c, map[string]string{"service": "flux-guard", "version": buildinfo.Version})
	})
}
