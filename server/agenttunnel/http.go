package agenttunnel

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/coder/websocket"

	"fluxguard/protocol"
	"fluxguard/server/helper/agent"
	"fluxguard/server/setting"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/httpx"
)

// Register 挂上代理出站隧道。令牌与上报相同。
func Register(router *gin.Engine, agents *agent.Store, hub *Hub) {
	if router == nil || hub == nil {
		return
	}
	router.GET(protocol.TunnelPath, func(c *gin.Context) {
		token := protocol.BearerToken(c.GetHeader("Authorization"))
		tenantID, ok := setting.TenantByIngestToken(token)
		if !ok || agents == nil {
			httpx.WriteCode(c, http.StatusUnauthorized, "agent.token.invalid", "代理令牌无效")
			return
		}
		agentCode := strings.TrimSpace(c.Query("agentCode"))
		machineID := strings.TrimSpace(c.Query("machineId"))
		agentID, err := agents.EnsureMachineAgent(tenantID, agentCode, machineID)
		if err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, err.Error())
			return
		}
		ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
			// 同源即可；代理不是浏览器，不依赖 Origin。
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		ws.SetReadLimit(int64(limits.TunnelMaxFrameBytes))
		hub.Serve(c.Request.Context(), tenantID, agentID, ws)
	})
}
