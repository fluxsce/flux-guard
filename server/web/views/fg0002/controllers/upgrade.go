package controllers

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0002/upgrade"

	"github.com/gin-gonic/gin"
)

// upgradeAgentsBody 是控制台下发代理自更新的 JSON 请求。
type upgradeAgentsBody struct {
	AgentCode   string   `json:"agentCode"`
	Version     string   `json:"version"`
	GoOS        string   `json:"goos"`
	GoArch      string   `json:"goarch"`
	AgentIDs    []string `json:"agentIds"`
	CanaryCount int      `json:"canaryCount"`
}

// QueryAgentReleases 列出本租户最近的代理升级（不含各台明细）。
func (c *AgentController) QueryAgentReleases(g *gin.Context) {
	p := middleware.Principal(g)
	code := strings.TrimSpace(request.Param(g, "agentCode"))
	limit := 0
	if raw := strings.TrimSpace(request.Param(g, "limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	out, err := upgrade.List(g.Request.Context(), c.db, p.TenantID, code, limit)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

// QueryAgentUpgradeStatuses 返回各代理最近一次仍需关注的升级状态，供列表展示。
func (c *AgentController) QueryAgentUpgradeStatuses(g *gin.Context) {
	p := middleware.Principal(g)
	out, err := upgrade.Statuses(g.Request.Context(), c.db, p.TenantID)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

// UpgradeAgents 使用中央 agents/ 包装，并向选中的代理分批下发自更新。
func (c *AgentController) UpgradeAgents(g *gin.Context) {
	p := middleware.Principal(g)
	var body upgradeAgentsBody
	if !request.Bind(g, &body) {
		return
	}
	code := strings.TrimSpace(body.AgentCode)
	version := strings.TrimSpace(body.Version)
	goos := strings.TrimSpace(body.GoOS)
	goarch := strings.TrimSpace(body.GoArch)
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	audit.Note(g, "", "AGENT", "", code, fmt.Sprintf("升级代理 %s %s %s/%s", code, version, goos, goarch))
	out, err := upgrade.Submit(g.Request.Context(), c.db, p.TenantID, p.UserID, upgrade.Input{
		AgentCode: code, Version: version, GoOS: goos, GoArch: goarch,
		AgentIDs: body.AgentIDs, CanaryCount: body.CanaryCount,
	})
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

// ContinueAgentRelease 继续下发同一发布中尚未升级的代理。
func (c *AgentController) ContinueAgentRelease(g *gin.Context) {
	p := middleware.Principal(g)
	releaseID := strings.TrimSpace(request.Param(g, "releaseId"))
	if releaseID == "" {
		var body struct {
			ReleaseID string `json:"releaseId"`
		}
		if !request.Bind(g, &body) {
			return
		}
		releaseID = strings.TrimSpace(body.ReleaseID)
	}
	if releaseID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少发布")
		return
	}
	audit.Note(g, "", "AGENT", releaseID, releaseID, "继续升级代理批次 "+releaseID)
	out, err := upgrade.Continue(g.Request.Context(), c.db, p.TenantID, p.UserID, releaseID)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}

// QueryAgentRelease 返回一次代理升级里各目标的命令状态与版本对照。
func (c *AgentController) QueryAgentRelease(g *gin.Context) {
	p := middleware.Principal(g)
	releaseID := strings.TrimSpace(request.Param(g, "releaseId"))
	if releaseID == "" {
		httpx.WriteErr(g, http.StatusBadRequest, "缺少发布")
		return
	}
	out, err := upgrade.Progress(g.Request.Context(), c.db, p.TenantID, releaseID)
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteOK(g, out)
}
