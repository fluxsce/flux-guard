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
	"fluxguard/server/web/views/fg0051/dao"
	dbx "github.com/fluxsce/dbx/db"
)

// LogController 查询系统操作日志。页面只读，不提供删除。
type LogController struct {
	logs *dao.LogDAO
}

// NewLogController 创建操作日志控制器。
func NewLogController(db *dbx.DB) *LogController {
	return &LogController{logs: dao.NewLogDAO(db)}
}

// QueryLogs 分页列出本租户的操作日志。
func (c *LogController) QueryLogs(g *gin.Context) {
	page := request.PageOf(g)
	rows, total, err := c.logs.PageLogs(middleware.Principal(g).TenantID, dao.LogQuery{
		UserName:   request.Param(g, "userName"),
		Action:     request.Param(g, "action"),
		ModuleCode: request.Param(g, "moduleCode"),
		ResultFlag: request.Param(g, "resultFlag"),
		FromTime:   request.Param(g, "fromTime"),
		ToTime:     request.Param(g, "toTime"),
	}, page)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载操作日志失败")
		return
	}
	request.WritePage(g, rows, total, page)
}

// GetLog 按审计主键返回一行。
func (c *LogController) GetLog(g *gin.Context) {
	auditID := strings.TrimSpace(request.Param(g, "auditId"))
	if auditID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少日志")
		return
	}
	row, err := c.logs.GetLog(g.Request.Context(), middleware.Principal(g).TenantID, auditID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteErr(g, http.StatusNotFound, "操作日志不存在")
		return
	}
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载操作日志失败")
		return
	}
	httpx.WriteOK(g, row)
}
