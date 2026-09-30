package service

import (
	"context"

	"fluxguard/server/alert/models"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

// PageChannels 按当前引擎分页返回渠道。
func (s *Service) PageChannels(tenantID string, q models.ChannelQuery, page dbx.Page) ([]models.Channel, int64, error) {
	return s.channels.Page(tenantID, q, page)
}

// SaveChannel 新增或修改渠道。
func (s *Service) SaveChannel(ctx context.Context, in models.ChannelInput, creating bool) (models.Channel, error) {
	return s.channels.Save(ctx, in, creating)
}

// DeleteChannel 删除渠道。
func (s *Service) DeleteChannel(ctx context.Context, channelID string) error {
	return s.channels.Delete(ctx, channelID)
}

// RequireChannel 确认渠道编码属于该租户。
func (s *Service) RequireChannel(ctx context.Context, tenantID, channelID string) error {
	return s.channels.Require(ctx, tenantID, channelID)
}

// PageTemplates 按当前引擎分页返回模板。
func (s *Service) PageTemplates(tenantID string, q models.TemplateQuery, page dbx.Page) ([]models.Template, int64, error) {
	return s.tpl.Page(tenantID, q, page)
}

// SaveTemplate 新增或修改模板。
func (s *Service) SaveTemplate(ctx context.Context, in models.TemplateInput, creating bool) (models.Template, error) {
	return s.tpl.Save(ctx, in, creating)
}

// DeleteTemplate 删除自定义模板。
func (s *Service) DeleteTemplate(ctx context.Context, templateID string) error {
	return s.tpl.Delete(ctx, templateID)
}

// PreviewTemplate 用样例数据渲染标题和正文。
func (s *Service) PreviewTemplate(tenantID string, in models.PreviewInput) (models.Preview, error) {
	return s.tpl.Preview(tenantID, in)
}

// ListLogs 返回最近预警日志。
func (s *Service) ListLogs(tenantID string) ([]models.Log, error) {
	return s.events.List(tenantID)
}

// PageLogs 按当前引擎分页返回预警日志。
func (s *Service) PageLogs(tenantID string, q models.LogQuery, page dbx.Page) ([]models.Log, int64, error) {
	return s.events.Page(tenantID, q, page)
}

// PageRules 按当前引擎分页返回规则。条件 JSON 原样返回。
func (s *Service) PageRules(tenantID string, q models.RuleQuery, page dbx.Page) ([]models.Rule, int64, error) {
	return s.rules.Page(tenantID, q, page)
}

// ListRules 返回某类型下的全部规则。
func (s *Service) ListRules(tenantID, alertType string) ([]models.Rule, error) {
	return s.rules.List(tenantID, alertType)
}

// GetRule 按编码和类型读取一条规则。
func (s *Service) GetRule(ctx context.Context, tenantID, ruleID, alertType string) (models.Rule, error) {
	return s.rules.Get(ctx, tenantID, ruleID, alertType)
}

// SaveRule 新增或修改规则。渠道留空表示只记录。本方法不解释条件。
func (s *Service) SaveRule(ctx context.Context, in models.RuleInput, creating bool) (models.Rule, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Rule{}, err
	}
	if err = s.channels.Require(ctx, sess.TenantID, in.ChannelID); err != nil {
		return models.Rule{}, err
	}
	return s.rules.Save(ctx, in, creating)
}

// DeleteRule 删除指定类型的规则。
func (s *Service) DeleteRule(ctx context.Context, ruleID, alertType string) error {
	return s.rules.Delete(ctx, ruleID, alertType)
}
