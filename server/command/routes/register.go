package commandroutes

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/command/service"
	"fluxguard/server/helper/agent"
	"fluxguard/server/helper/agentrelease"
	"fluxguard/server/helper/appfile"
	"fluxguard/server/helper/applist"
	"fluxguard/server/helper/apprelease"
	"fluxguard/server/setting"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/request"
	"github.com/gin-gonic/gin"
)

// Register 挂上代理取令和回执。控制台下发不在这里。
func Register(router *gin.Engine, agents *agent.Store, svc *service.Service) {
	if svc == nil || router == nil {
		return
	}
	router.GET(protocol.WaitPath, func(c *gin.Context) {
		tenantID, agentID, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		deadline := time.Now().Add(holdFor(tenantID))
		for {
			call, claimed, err := svc.ClaimNext(c.Request.Context(), tenantID, agentID)
			if err != nil {
				httpx.WriteErr(c, http.StatusInternalServerError, "领取命令失败")
				return
			}
			if claimed {
				httpx.WriteOK(c, map[string]any{"command": call})
				return
			}
			remain := time.Until(deadline)
			if remain <= 0 || c.Request.Context().Err() != nil {
				httpx.WriteOK(c, map[string]any{"command": nil})
				return
			}
			svc.Wait(agentID, c.Request.Context(), remain)
		}
	})
	router.GET(protocol.ArtifactPath, func(c *gin.Context) {
		tenantID, agentID, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		releaseID := strings.TrimSpace(c.Query("releaseId"))
		if !svc.OwnsRelease(c.Request.Context(), tenantID, agentID, releaseID) {
			httpx.WriteCode(c, http.StatusNotFound, "release.missing", "找不到这件制品")
			return
		}
		file, err := agentrelease.Open(tenantID, releaseID)
		if err != nil {
			file, err = apprelease.Open(tenantID, releaseID)
		}
		if err != nil {
			httpx.WriteCode(c, http.StatusNotFound, "release.missing", "找不到这件制品")
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			httpx.WriteCode(c, http.StatusNotFound, "release.missing", "找不到这件制品")
			return
		}
		c.DataFromReader(http.StatusOK, info.Size(), "application/octet-stream", file, nil)
	})
	router.POST(protocol.ResultPath, func(c *gin.Context) {
		tenantID, agentID, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		var body protocol.CommandResult
		if !request.Bind(c, &body) {
			return
		}
		if err := svc.FinishOwned(c.Request.Context(), tenantID, agentID, body); err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, err.Error())
			return
		}
		httpx.WriteOK(c, map[string]string{"commandId": body.CommandID})
	})
	router.POST(protocol.FilePushPath, func(c *gin.Context) {
		tenantID, _, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		commandID := strings.TrimSpace(c.PostForm("commandId"))
		if commandID == "" {
			httpx.WriteErr(c, http.StatusBadRequest, "缺少命令")
			return
		}
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, "请上传文件")
			return
		}
		defer file.Close()
		if header.Size > limits.AppFileMaxBytes {
			httpx.WriteErr(c, http.StatusBadRequest, "文件超过代下上限（2GiB）")
			return
		}
		if err := appfile.SaveWithProgress(tenantID, commandID, file, header.Size); err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, err.Error())
			return
		}
		name := filepath.Base(strings.TrimSpace(header.Filename))
		if name == "" || name == "." {
			name = commandID
		}
		if path, err := appfile.Path(tenantID, commandID); err == nil {
			_ = os.WriteFile(path+".name", []byte(name), 0o644)
		}
		httpx.WriteOK(c, map[string]string{"commandId": commandID})
	})
	router.POST(protocol.FileListPushPath, func(c *gin.Context) {
		tenantID, agentID, ok := agentIdentity(c, agents)
		if !ok {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
		var body struct {
			CommandID string                 `json:"commandId"`
			Instance  string                 `json:"instance"`
			Artifacts []protocol.AppArtifact `json:"artifacts"`
		}
		if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, "清单格式无效")
			return
		}
		commandID := strings.TrimSpace(body.CommandID)
		if commandID == "" {
			httpx.WriteErr(c, http.StatusBadRequest, "缺少命令")
			return
		}
		instance, err := svc.ListCommandMeta(c.Request.Context(), tenantID, agentID, commandID)
		if err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, err.Error())
			return
		}
		if inst := strings.TrimSpace(body.Instance); inst != "" && inst != instance {
			httpx.WriteErr(c, http.StatusBadRequest, "实例与命令不一致")
			return
		}
		if err := applist.Save(tenantID, commandID, body.Artifacts); err != nil {
			httpx.WriteErr(c, http.StatusBadRequest, err.Error())
			return
		}
		httpx.WriteOK(c, map[string]string{"commandId": commandID, "instance": instance})
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

func holdFor(tenantID string) time.Duration {
	sec := limits.NormalizeAPITimeoutSec(request.Timeout(tenantID).RequestTimeoutSeconds)
	return limits.CapHold(limits.CommandWaitHold, sec)
}
