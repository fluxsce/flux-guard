package controllers

import (
	"net/http"
	"strings"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/request"

	"github.com/gin-gonic/gin"

	"fluxguard/server/alert"
)

// ChannelController 通知渠道查询、维护、测试和预览。
type ChannelController struct {
	Alert *alert.Service
}

// NewChannelController 创建渠道控制器。
func NewChannelController(svc *alert.Service) *ChannelController {
	return &ChannelController{Alert: svc}
}

func (c *ChannelController) ready(g *gin.Context) bool {
	if c == nil || c.Alert == nil {
		httpx.WriteErr(g, http.StatusServiceUnavailable, "预警服务未启用")
		return false
	}
	return true
}

// QueryChannels 列出本租户渠道。密钥不回传。
func (c *ChannelController) QueryChannels(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	page := request.PageOf(g)
	rows, total, err := c.Alert.PageChannels(middleware.Principal(g).TenantID, alert.ChannelQuery{
		ChannelName: request.Param(g, "channelName"),
		ChannelKind: request.Param(g, "channelKind"),
		StatusFlag:  request.Param(g, "statusFlag"),
	}, page)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	request.WritePage(g, rows, total, page)
}

// CreateChannel 新增渠道。
func (c *ChannelController) CreateChannel(g *gin.Context) {
	c.save(g, true)
}

// UpdateChannel 修改渠道。密钥留空表示不改。
func (c *ChannelController) UpdateChannel(g *gin.Context) {
	c.save(g, false)
}

func (c *ChannelController) save(g *gin.Context, creating bool) {
	if !c.ready(g) {
		return
	}
	var body alert.ChannelInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "CHANNEL", body.ChannelID, body.ChannelName, "")
	saved, err := c.Alert.SaveChannel(g.Request.Context(), body, creating)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	verb := "修改通知渠道 "
	if creating {
		verb = "新增通知渠道 "
	}
	detail := verb + audit.Named(saved.ChannelName, saved.ChannelID)
	if saved.ChannelKind != "" {
		detail += "，种类 " + saved.ChannelKind
	}
	if strings.TrimSpace(body.Secret) != "" {
		detail += "，并更新了密钥"
	}
	audit.Note(g, "", "CHANNEL", saved.ChannelID, saved.ChannelName, detail)
	httpx.WriteOK(g, saved)
}

// DeleteChannel 删除渠道。
func (c *ChannelController) DeleteChannel(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body alert.IDInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "CHANNEL", body.ChannelID, body.ChannelID, "")
	if err := c.Alert.DeleteChannel(g.Request.Context(), body.ChannelID); err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	audit.Note(g, "", "CHANNEL", body.ChannelID, body.ChannelID, "删除通知渠道 "+body.ChannelID)
	httpx.WriteOK(g, map[string]any{"deleted": true})
}

// TestChannel 向该渠道发送一条样例消息。
func (c *ChannelController) TestChannel(g *gin.Context) {
	if !c.ready(g) {
		return
	}
	var body alert.IDInput
	if !request.Bind(g, &body) {
		return
	}
	audit.Note(g, "", "CHANNEL", body.ChannelID, body.ChannelID, "")
	row, err := c.Alert.TestChannel(middleware.Principal(g).TenantID, body.ChannelID)
	if err != nil {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return
	}
	detail := "测试通知渠道 " + body.ChannelID
	if row.SendStatus != "" {
		detail += "，结果 " + row.SendStatus
	}
	audit.Note(g, "", "CHANNEL", body.ChannelID, body.ChannelID, detail)
	httpx.WriteOK(g, row)
}
