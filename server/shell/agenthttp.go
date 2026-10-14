package shell

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fluxguard/protocol"
	"fluxguard/server/helper/agent"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/setting"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/request"
)

// holdFor 让等按键的时间短于写超时，避免连接被服务器掐断后空转。
func holdFor(tenantID string) time.Duration {
	sec := limits.NormalizeAPITimeoutSec(request.Timeout(tenantID).RequestTimeoutSeconds)
	return limits.CapHold(limits.ShellInputHold, sec)
}

// RegisterAgent 挂上代理主动连接的终端通道。令牌与上报相同，不走控制台会话。
func RegisterAgent(router *gin.Engine, agents *agent.Store, shells *Store) {
	if router == nil || shells == nil {
		return
	}
	router.GET(protocol.ShellSessionsPath, func(c *gin.Context) {
		tenantID, agentID, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		list, err := shells.List(c.Request.Context(), tenantID, agentID)
		if err != nil {
			httpx.WriteErr(c, http.StatusInternalServerError, "读取终端失败")
			return
		}
		httpx.WriteOK(c, list)
	})
	router.GET(protocol.ShellInputPath, func(c *gin.Context) {
		tenantID, agentID, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		in, err := shells.TakeInput(c.Request.Context(), tenantID, agentID, c.Query("shellId"), holdFor(tenantID))
		if err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, err.Error())
			return
		}
		httpx.WriteOK(c, in)
	})
	router.POST(protocol.ShellOutputPath, func(c *gin.Context) {
		tenantID, agentID, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		var body struct {
			ShellID string `json:"shellId"`
			Data    string `json:"data"`
		}
		if !request.Bind(c, &body) {
			return
		}
		if err := shells.PutOutput(c.Request.Context(), tenantID, agentID, body.ShellID, body.Data); err != nil {
			httpx.WriteFail(c, http.StatusBadRequest, err)
			return
		}
		httpx.WriteOK(c, map[string]string{"shellId": body.ShellID})
	})
}

func agentIdentity(c *gin.Context, agents *agent.Store) (string, string, bool) {
	token := protocol.BearerToken(c.GetHeader("Authorization"))
	tenantID, ok := setting.TenantByIngestToken(token)
	if !ok || agents == nil {
		httpx.WriteCode(c, http.StatusUnauthorized, "agent.token.invalid", "代理令牌无效")
		return "", "", false
	}
	agentID, err := agents.EnsureMachineAgent(tenantID, c.Query("agentCode"), c.Query("machineId"))
	if err != nil {
		httpx.WriteErr(c, http.StatusBadRequest, err.Error())
		return "", "", false
	}
	return tenantID, agentID, true
}
