package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/request"
)

// EventController 列出预警日志。发送结果写在同一行。
type EventController struct {
	Alert *alert.Service
}

// NewEventController 创建日志控制器。
func NewEventController(svc *alert.Service) *EventController {
	return &EventController{Alert: svc}
}

func (c *EventController) ready(g *gin.Context) bool {
	if c == nil || c.Alert == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "预警服务未启用")
		return false
	}
	return true
}

// QueryLogs 列出本租户预警日志。
func (c *EventController) QueryLogs(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	page := request.PageOf(g)
	rows, total, err := c.Alert.PageLogs(middleware.Principal(g).TenantID, alert.LogQuery{
		AlertTitle: request.Param(g, "alertTitle"),
		AlertLevel: request.Param(g, "alertLevel"),
		AlertType:  request.Param(g, "alertType"),
		SendStatus: request.Param(g, "sendStatus"),
	}, page)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	request.WritePage(g, rows, total, page)
}
