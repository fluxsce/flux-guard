package controllers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fluxguard/server/store"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0006/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var errSeriesSpan = errors.New("时间段无效")

// JavaController Java 应用当前占用和历史曲线。
type JavaController struct {
	db *dbx.DB
}

// NewJavaController 创建 Java 监控控制器。
func NewJavaController(db *dbx.DB) *JavaController {
	return &JavaController{db: db}
}

// QueryApps 返回每个 Java 进程的当前堆、线程和回收累计。
func (c *JavaController) QueryApps(g *gin.Context) {
	p := middleware.Principal(g)
	page := request.PageOf(g)
	rows, total, err := dao.List(c.db, p.TenantID, dao.JavaFilter{
		AppName:  request.Param(g, "appName"),
		HostName: request.Param(g, "hostName"),
	}, page)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载 Java 应用失败")
		return
	}
	request.WritePage(g, rows, total, page)
}

// QueryDetail 返回一个进程最新一批回收器。
func (c *JavaController) QueryDetail(g *gin.Context) {
	p := middleware.Principal(g)
	agentID := strings.TrimSpace(request.Param(g, "agentId"))
	if agentID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少代理")
		return
	}
	httpx.WriteOK(g, dao.Detail(c.db, p.TenantID, agentID))
}

// QuerySeries 返回一个进程在时间段内的曲线。未传起止时间时返回最近一批点。
func (c *JavaController) QuerySeries(g *gin.Context) {
	p := middleware.Principal(g)
	agentID := strings.TrimSpace(request.Param(g, "agentId"))
	if agentID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少代理")
		return
	}
	span, err := seriesSpan(g)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, dao.Series(c.db, p.TenantID, agentID, span))
}

func seriesSpan(g *gin.Context) (dao.SeriesSpan, error) {
	fromText := strings.TrimSpace(request.Param(g, "from"))
	toText := strings.TrimSpace(request.Param(g, "to"))
	if fromText == "" && toText == "" {
		return dao.SeriesSpan{}, nil
	}
	if fromText == "" || toText == "" {
		return dao.SeriesSpan{}, errSeriesSpan
	}
	from, ok := store.ParseTime(fromText)
	if !ok {
		return dao.SeriesSpan{}, errSeriesSpan
	}
	to, ok := store.ParseTime(toText)
	if !ok || !to.After(from) || to.Sub(from) > 7*24*time.Hour {
		return dao.SeriesSpan{}, errSeriesSpan
	}
	return dao.SeriesSpan{From: from, To: to}, nil
}
