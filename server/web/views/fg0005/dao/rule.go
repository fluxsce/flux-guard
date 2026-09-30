package dao

import (
	"context"
	"encoding/json"
	"strings"

	"fluxguard/server/alert"
	"fluxguard/server/alert/validate"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0005/models"
	dbx "github.com/fluxsce/dbx/db"
)

const hostAlertType = "host"

// RuleFilter 是主机预警规则列表的筛选条件。
type RuleFilter struct {
	RuleName   string
	MetricCode string
	LevelCode  string
	StatusFlag string
}

type hostCondition struct {
	MetricCode        string  `json:"metricCode"`
	CompareOp         string  `json:"compareOp"`
	ThresholdValue    float64 `json:"thresholdValue"`
	DurationSec       int     `json:"durationSec"`
	AgentID           string  `json:"agentId"`
	WaitMinutes       int     `json:"waitMinutes"`
	EscalateChannelID string  `json:"escalateChannelId"`
}

// PageRules 按当前引擎分页返回主机预警规则。
func PageRules(db *dbx.DB, tenantID string, q RuleFilter, page dbx.Page) ([]models.HostRule, int64, error) {
	alerts := alert.Current()
	if alerts == nil {
		alerts = alert.Open(db)
	}
	if alerts == nil {
		return nil, 0, validate.ErrText("预警服务未打开")
	}
	match := ""
	if code := strings.TrimSpace(q.MetricCode); code != "" {
		match = `"metricCode":"` + code + `"`
	}
	rows, total, err := alerts.PageRules(tenantID, alert.RuleQuery{
		RuleName: q.RuleName, AlertType: hostAlertType, LevelCode: q.LevelCode,
		StatusFlag: q.StatusFlag, ConditionMatch: match,
	}, page)
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.HostRule, 0, len(rows))
	for _, row := range rows {
		item, ok := flattenHost(row)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, total, nil
}

// ListRules 返回租户下全部主机预警规则，供扫描使用。
func ListRules(db *dbx.DB, tenantID string) ([]models.HostRule, error) {
	alerts := alert.Current()
	if alerts == nil {
		alerts = alert.Open(db)
	}
	if alerts == nil {
		return nil, validate.ErrText("预警服务未打开")
	}
	rows, err := alerts.ListRules(tenantID, hostAlertType)
	if err != nil {
		return nil, err
	}
	out := make([]models.HostRule, 0, len(rows))
	for _, row := range rows {
		item, ok := flattenHost(row)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// SaveRule 新增或修改主机预警规则。条件写入通用规则表，渠道留空表示只记录。
func SaveRule(ctx context.Context, db *dbx.DB, alerts *alert.Service, in models.HostRuleInput, creating bool) (models.HostRule, error) {
	if alerts == nil {
		alerts = alert.Open(db)
	}
	if alerts == nil {
		return models.HostRule{}, validate.ErrText("预警服务未打开")
	}
	cond, err := packHost(in)
	if err != nil {
		return models.HostRule{}, err
	}
	sess, err := request.Require(ctx)
	if err != nil {
		return models.HostRule{}, err
	}
	if err = alerts.RequireChannel(ctx, sess.TenantID, cond.EscalateChannelID); err != nil {
		return models.HostRule{}, err
	}
	text, err := json.Marshal(cond)
	if err != nil {
		return models.HostRule{}, err
	}
	saved, err := alerts.SaveRule(ctx, alert.RuleInput{
		RuleID: in.RuleID, RuleName: in.RuleName, AlertType: hostAlertType, LevelCode: in.LevelCode,
		ChannelID: in.ChannelID, SilenceStart: in.SilenceStart, SilenceEnd: in.SilenceEnd,
		StatusFlag: in.StatusFlag, ActiveFlag: in.ActiveFlag, NoteText: in.NoteText,
		ConditionText: string(text),
	}, creating)
	if err != nil {
		return models.HostRule{}, err
	}
	item, ok := flattenHost(saved)
	if !ok {
		return models.HostRule{}, validate.ErrText("规则条件无效")
	}
	return item, nil
}

// DeleteRule 删除主机预警规则。
func DeleteRule(ctx context.Context, db *dbx.DB, ruleID string) error {
	alerts := alert.Current()
	if alerts == nil {
		alerts = alert.Open(db)
	}
	if alerts == nil {
		return validate.ErrText("预警服务未打开")
	}
	return alerts.DeleteRule(ctx, ruleID, hostAlertType)
}

func packHost(in models.HostRuleInput) (hostCondition, error) {
	if !validMetric(in.MetricCode) {
		return hostCondition{}, validate.ErrText("指标编码无效")
	}
	if !validOp(in.CompareOp) {
		return hostCondition{}, validate.ErrText("比较符无效")
	}
	if in.DurationSec < 0 {
		return hostCondition{}, validate.ErrText("持续秒数无效")
	}
	if in.WaitMinutes < 0 {
		return hostCondition{}, validate.ErrText("等待分钟无效")
	}
	return hostCondition{
		MetricCode: in.MetricCode, CompareOp: in.CompareOp, ThresholdValue: in.ThresholdValue,
		DurationSec: in.DurationSec, AgentID: strings.TrimSpace(in.AgentID),
		WaitMinutes: in.WaitMinutes, EscalateChannelID: strings.TrimSpace(in.EscalateChannelID),
	}, nil
}

func flattenHost(row alert.Rule) (models.HostRule, bool) {
	var cond hostCondition
	if err := json.Unmarshal([]byte(row.ConditionText), &cond); err != nil {
		return models.HostRule{}, false
	}
	if !validMetric(cond.MetricCode) || !validOp(cond.CompareOp) {
		return models.HostRule{}, false
	}
	return models.HostRule{
		RuleID: row.RuleID, RuleName: row.RuleName, MetricCode: cond.MetricCode, CompareOp: cond.CompareOp,
		ThresholdValue: cond.ThresholdValue, DurationSec: cond.DurationSec, LevelCode: row.LevelCode,
		AgentID: cond.AgentID, ChannelID: row.ChannelID, SilenceStart: row.SilenceStart, SilenceEnd: row.SilenceEnd,
		WaitMinutes: cond.WaitMinutes, EscalateChannelID: cond.EscalateChannelID, StatusFlag: row.StatusFlag,
		ActiveFlag: row.ActiveFlag, NoteText: row.NoteText, AddTime: row.AddTime, AddWho: row.AddWho,
		EditTime: row.EditTime, EditWho: row.EditWho, CurrentVersion: row.CurrentVersion, OprSeqFlag: row.OprSeqFlag,
	}, true
}
