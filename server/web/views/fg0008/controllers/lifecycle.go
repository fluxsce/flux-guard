package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/protocol"
	"fluxguard/server/command"
	cmdmodels "fluxguard/server/command/models"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0008/dao"
)

type runtimeOpBody struct {
	AgentID      string `json:"agentId"`
	InstanceName string `json:"instanceName"`
	Op           string `json:"op"` // start | stop | restart
}

// RuntimeControl 对一套运行时下发启动 / 停止 / 重启（Control 面，不经 Session）。
func (c *AppController) RuntimeControl(g *gin.Context) {
	p := middleware.Principal(g)
	var body runtimeOpBody
	if !request.Bind(g, &body) {
		return
	}
	agentID := strings.TrimSpace(body.AgentID)
	instance := strings.TrimSpace(body.InstanceName)
	op := strings.ToLower(strings.TrimSpace(body.Op))
	if agentID == "" || instance == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少运行时")
		return
	}
	var action string
	switch op {
	case "start":
		action = protocol.ActionAppStart
	case "stop":
		action = protocol.ActionAppStop
	case "restart":
		action = protocol.ActionAppRestart
	default:
		httpx.WriteErr(g, http.StatusBadRequest, "操作无效，须为 start / stop / restart")
		return
	}
	mode, err := dao.SiteMode(g.Request.Context(), c.db, p.TenantID, agentID, instance)
	if err != nil {
		writeMissing(g, err, "找不到运行时")
		return
	}
	if mode == "files" || mode == "static" {
		httpx.WriteErr(g, http.StatusBadRequest, "当前模式无进程启停")
		return
	}
	svc := command.Current()
	if svc == nil {
		httpx.WriteErr(g, http.StatusBadRequest, "命令通道未启用")
		return
	}
	audit.Note(g, "", "APP", instance, instance, "运行时"+op+" "+instance)
	view, err := svc.Dispatch(g.Request.Context(), p.TenantID, p.UserID, command.DispatchInput{
		AgentID: agentID, Action: action, Instance: instance,
	})
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, view)
}

// QueryCommand 查询一条命令进度（启停与换包共用）。
func (c *AppController) QueryCommand(g *gin.Context) {
	p := middleware.Principal(g)
	commandID := strings.TrimSpace(request.Param(g, "commandId"))
	if commandID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少命令")
		return
	}
	svc := command.Current()
	if svc == nil {
		httpx.WriteErr(g, http.StatusBadRequest, "命令通道未启用")
		return
	}
	view, err := svc.Get(g.Request.Context(), p.TenantID, commandID)
	if err != nil {
		if errors.Is(err, cmdmodels.ErrCommandGone) {
			httpx.WriteErr(g, http.StatusBadRequest, "找不到命令")
			return
		}
		writeMissing(g, err, "找不到命令")
		return
	}
	httpx.WriteOK(g, view)
}
