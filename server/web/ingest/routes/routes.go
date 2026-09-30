package ingestroutes

import (
	"github.com/gin-gonic/gin"

	"fluxguard/protocol"
	"fluxguard/server/agent"
	"fluxguard/server/web/ingest/controllers"
	"fluxguard/server/web/ingest/service"
	"fluxguard/server/web/routes"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	routes.RegisterEntry(Init)
}

// Init 挂上代理上报入口。路径在 protocol，不进模块号。
func Init(router *gin.Engine, db *dbx.DB) {
	ctrl := controllers.NewBatchController(agent.Open(db), service.Open(db))
	router.POST(protocol.IngestPath, ctrl.Accept)
}
