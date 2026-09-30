package controllers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0052/dao"
	dbx "github.com/fluxsce/dbx/db"
)

// LogController 查询代理操作日志。页面只读，不提供删除。
type LogController struct {
	logs *dao.LogDAO
}

// NewLogController 创建代理日志控制器。
func NewLogController(db *dbx.DB) *LogController {
	return &LogController{logs: dao.NewLogDAO(db)}
}

// QueryLogs 分页列出本租户的代理命令与终端账本。
func (c *LogController) QueryLogs(g *gin.Context) {
	page := request.PageOf(g)
	rows, total, err := c.logs.PageLogs(middleware.Principal(g).TenantID, dao.LogQuery{
		AgentName:  request.Param(g, "agentName"),
		UserName:   request.Param(g, "userName"),
		Source:     request.Param(g, "source"),
		EventType:  request.Param(g, "eventType"),
		StatusFlag: request.Param(g, "statusFlag"),
		FromTime:   request.Param(g, "fromTime"),
		ToTime:     request.Param(g, "toTime"),
	}, page)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载代理日志失败")
		return
	}
	request.WritePage(g, rows, total, page)
}

// GetLog 按来源和业务主键返回一行。
func (c *LogController) GetLog(g *gin.Context) {
	source := strings.TrimSpace(request.Param(g, "source"))
	logID := strings.TrimSpace(request.Param(g, "logId"))
	if source == "" || logID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少日志")
		return
	}
	row, err := c.logs.GetLog(g.Request.Context(), middleware.Principal(g).TenantID, source, logID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteErr(g, http.StatusNotFound, "代理日志不存在")
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载代理日志失败")
		return
	}
	httpx.WriteOK(g, row)
}
