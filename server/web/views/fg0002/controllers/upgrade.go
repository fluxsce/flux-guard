package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0002/upgrade"

	"github.com/gin-gonic/gin"
)

// UpgradeAgents 上传代理安装包，并向选中的代理分批下发自更新。
func (c *AgentController) UpgradeAgents(g *gin.Context) {
	p := middleware.Principal(g)
	file, header, err := g.Request.FormFile("file")
	if err != nil {
		httpx.WriteErr(g, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()
	if header.Size > 512<<20 {
		httpx.WriteErr(g, http.StatusBadRequest, "文件超过 512MB")
		return
	}
	code := strings.TrimSpace(g.PostForm("agentCode"))
	version := strings.TrimSpace(g.PostForm("version"))
	goos := strings.TrimSpace(g.PostForm("goos"))
	goarch := strings.TrimSpace(g.PostForm("goarch"))
	agentIDs := splitIDs(g.PostForm("agentIds"))
	canary := upgrade.ParseCanary(g.PostForm("canaryCount"))
	audit.Note(g, "", "AGENT", "", code, fmt.Sprintf("升级代理 %s %s %s/%s", code, version, goos, goarch))
	out, err := upgrade.Submit(g.Request.Context(), c.db, p.TenantID, p.UserID, upgrade.Input{
		AgentCode: code, Version: version, GoOS: goos, GoArch: goarch,
		File: header.Filename, Body: file, AgentIDs: agentIDs, CanaryCount: canary,
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

func splitIDs(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
