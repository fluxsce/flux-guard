package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"

	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
	"fluxguard/server/web/views/fg0005/dao"
	"fluxguard/server/web/views/fg0005/models"
	dbx "github.com/fluxsce/dbx/db"
)

var errSeriesSpan = errors.New("时间段无效")

// HostController 主机当前占用。
type HostController struct {
	db *dbx.DB
}

// NewHostController 创建主机监控控制器。
func NewHostController(db *dbx.DB) *HostController {
	return &HostController{db: db}
}

// QueryHosts 返回每台机器的当前 CPU、内存、磁盘和流量。
func (c *HostController) QueryHosts(g *gin.Context) {
	p := middleware.Principal(g)
	page := request.PageOf(g)
	rows, total, err := dao.List(c.db, p.TenantID, dao.HostFilter{
		HostName:  request.Param(g, "hostName"),
		IPAddress: request.Param(g, "ipAddress"),
	}, page)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载主机失败")
		return
	}
	request.WritePage(g, rows, total, page)
}

// QueryHostDetail 返回一台机器的磁盘、磁盘 IO、网卡和单核当前值。
func (c *HostController) QueryHostDetail(g *gin.Context) {
	p := middleware.Principal(g)
	agentID := strings.TrimSpace(request.Param(g, "agentId"))
	if agentID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少代理")
		return
	}
	httpx.WriteOK(g, dao.Detail(c.db, p.TenantID, agentID))
}

// QueryHostSeries 返回一台机器在时间段内的曲线。未传起止时间时返回最近一批点。
func (c *HostController) QueryHostSeries(g *gin.Context) {
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

// QueryRules 列出本租户的主机预警规则。条件由本模块从通用规则里拆出。
func (c *HostController) QueryRules(g *gin.Context) {
	p := middleware.Principal(g)
	page := request.PageOf(g)
	rows, total, err := dao.PageRules(c.db, p.TenantID, dao.RuleFilter{
		RuleName:   request.Param(g, "ruleName"),
		MetricCode: request.Param(g, "metricCode"),
		LevelCode:  request.Param(g, "levelCode"),
		StatusFlag: request.Param(g, "statusFlag"),
	}, page)
	if err != nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "加载规则失败")
		return
	}
	request.WritePage(g, rows, total, page)
}

// CreateRule 新增主机预警规则。
func (c *HostController) CreateRule(g *gin.Context) {
	c.saveRule(g, true)
}

// UpdateRule 修改主机预警规则。
func (c *HostController) UpdateRule(g *gin.Context) {
	c.saveRule(g, false)
}

func (c *HostController) saveRule(g *gin.Context, creating bool) {
	var body models.HostRuleInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "RULE", body.RuleID, body.RuleName, "")
	saved, err := dao.SaveRule(g.Request.Context(), c.db, alert.Open(c.db), body, creating)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	verb := "修改预警规则 "
	if creating {
		verb = "新增预警规则 "
	}
	detail := verb + audit.Named(saved.RuleName, saved.RuleID)
	if saved.MetricCode != "" {
		detail += fmt.Sprintf("，%s %s %g", saved.MetricCode, saved.CompareOp, saved.ThresholdValue)
	}
	audit.Note(g, "", "RULE", saved.RuleID, saved.RuleName, detail)
	httpx.WriteOK(g, saved)
}

// DeleteRule 删除主机预警规则。
func (c *HostController) DeleteRule(g *gin.Context) {
	var body alert.IDInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "RULE", body.RuleID, body.RuleID, "")
	if err := dao.DeleteRule(g.Request.Context(), c.db, body.RuleID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "RULE", body.RuleID, body.RuleID, "删除预警规则 "+body.RuleID)
	httpx.WriteOK(g, map[string]any{"deleted": true})
}
