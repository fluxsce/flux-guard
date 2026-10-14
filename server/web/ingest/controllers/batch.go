package controllers

import (
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/request"
	"net/http"

	"github.com/gin-gonic/gin"

	"strings"

	"fluxguard/protocol"
	"fluxguard/server/agent"
	"fluxguard/server/setting"
	"fluxguard/server/web/ingest/service"
)

// BatchController 接收代理上报。
type BatchController struct {
	Agents *agent.Store
	Ingest *service.Service
}

// NewBatchController 创建上报控制器。
func NewBatchController(agents *agent.Store, ingestSvc *service.Service) *BatchController {
	return &BatchController{Agents: agents, Ingest: ingestSvc}
}

// Accept 校验令牌、解析代理，再交给服务写入。
func (c *BatchController) Accept(g *gin.Context) {
	token := protocol.BearerToken(g.Request.Header.Get("Authorization"))
	tenantID, ok := setting.TenantByIngestToken(token)
	if !ok {
		httpx.WriteCode(g, http.StatusUnauthorized, "agent.token.invalid", "代理令牌无效")
		return
	}
	var body protocol.Request
	if !request.Bind(g, &body) {
		return
	}
	agentID, err := resolveAgent(c.Agents, tenantID, token, body)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	result, err := c.Ingest.Accept(g.Request.Context(), body.Batch(tenantID, agentID))
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	c.Agents.TouchAgent(tenantID, agentID)
	httpx.WriteOK(g, result)
}

// resolveAgent 用机器编号把同一租户里的多台主机分成多行。
// Java 用服务名加实例名。重启后进程号变了仍落在同一行。没有机器编号时，仍按登记令牌或种类共用一行。
func resolveAgent(agents *agent.Store, tenantID, token string, body protocol.Request) (string, error) {
	if !protocol.KnownAgent(body.AgentCode) {
		return "", agent.ErrAgentIdentity
	}
	if machineID := reportingMachineID(body); machineID != "" {
		return agents.EnsureMachineAgent(tenantID, body.AgentCode, machineID)
	}
	if rowTenant, id, found := agents.AgentByToken(token); found && rowTenant == tenantID {
		return id, nil
	}
	return agents.EnsureReportingAgent(tenantID, body.AgentCode)
}

// reportingMachineID 取出这一批用来分行的机器编号。
// 主机用操作系统编号。Java 用「服务名@实例」：实例优先取配置名，否则用主机名或 Pod 名。
// 进程号不参与编号，同一实例重启仍是这一行。没有主机名时才退回进程号。
func reportingMachineID(body protocol.Request) string {
	if body.Host != nil {
		if id := strings.TrimSpace(body.Host.MachineID); id != "" {
			return id
		}
	}
	if body.Java == nil {
		return ""
	}
	return javaInstanceID(body.Java)
}

func javaInstanceID(java *protocol.Java) string {
	if java == nil {
		return ""
	}
	app := strings.TrimSpace(java.AppName)
	if app == "" {
		app = "java"
	}
	slot := strings.TrimSpace(java.InstanceName)
	if slot == "" {
		slot = strings.TrimSpace(java.HostName)
	}
	if slot == "" {
		return clipMachineID(strings.TrimSpace(java.ProcessID))
	}
	return clipMachineID(app + "@" + slot)
}

func clipMachineID(id string) string {
	if len(id) <= 64 {
		return id
	}
	return id[:64]
}
