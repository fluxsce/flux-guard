package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fluxguard/common/logger"
	"fluxguard/server/alert"
	"fluxguard/server/alert/validate"
	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0005/models"
	dbx "github.com/fluxsce/dbx/db"
)

// hostTarget 是一次判定面对的主机。LastSeen 为空表示从未上报。
type hostTarget struct {
	AgentID  string
	HostName string
	LastSeen any
}

// Evaluate 按已启用的主机预警规则检查最新指标。越阈、持续和静默在本模块算完，再把事件交给预警按渠道发送。
func Evaluate(ctx context.Context, db *dbx.DB, alerts *alert.Service) {
	if db == nil || alerts == nil {
		return
	}
	rows, err := db.Query(ctx, `SELECT tenantId FROM FG_TENANT WHERE activeFlag = 'Y'`, nil)
	if err != nil {
		logger.Error(ctx, "主机预警: %v", err)
		return
	}
	defer rows.Close()
	var tenants []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			logger.Error(ctx, "主机预警: %v", err)
			return
		}
		if tenantID != "" {
			tenants = append(tenants, tenantID)
		}
	}
	if err := rows.Err(); err != nil {
		logger.Error(ctx, "主机预警: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, tenantID := range tenants {
		if ctx.Err() != nil {
			return
		}
		if err := evaluateTenant(ctx, db, alerts, tenantID, now); err != nil && ctx.Err() == nil {
			logger.Error(ctx, "主机预警 %s: %v", tenantID, err)
		}
	}
}

func evaluateTenant(ctx context.Context, db *dbx.DB, alerts *alert.Service, tenantID string, now time.Time) error {
	rules, err := ListRules(db, tenantID)
	if err != nil {
		return err
	}
	hosts, err := listHosts(ctx, db, tenantID)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return err
		}
		if rule.StatusFlag != "Y" || rule.ActiveFlag != "Y" {
			continue
		}
		for _, host := range hosts {
			if err := ctx.Err(); err != nil {
				return err
			}
			if rule.AgentID != "" && rule.AgentID != host.AgentID {
				continue
			}
			value, ok := metricValue(ctx, db, tenantID, host, rule.MetricCode, now)
			if !ok {
				continue
			}
			if err := submitHost(ctx, db, alerts, tenantID, rule, host, value, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func submitHost(ctx context.Context, db *dbx.DB, alerts *alert.Service, tenantID string, rule models.HostRule, host hostTarget, value float64, now time.Time) error {
	current, found, err := openIncident(ctx, db, tenantID, rule.RuleID, host.AgentID)
	if err != nil {
		return err
	}
	hit := breached(rule.CompareOp, value, rule.ThresholdValue)
	message := validate.Clip(fmt.Sprintf("%s %s 当前 %s，阈值 %s %s", host.HostName, metricName(rule.MetricCode), formatNum(value), opLabel(rule.CompareOp), formatNum(rule.ThresholdValue)), 500)
	silenced := ruleSilenced(rule, now)
	if hit {
		status := "pending"
		if !found {
			if rule.DurationSec <= 0 {
				status = "firing"
			}
			item := newIncident(rule, host, value, status, message)
			saved, err := insertIncident(ctx, db, tenantID, item, now)
			if err != nil || status != "firing" {
				return err
			}
			return markFire(ctx, db, alerts, tenantID, rule, host, saved, value, message, rule.ChannelID, silenced, now)
		}
		current.CurrentValue = value
		current.MessageText = message
		current.LevelCode = rule.LevelCode
		if current.StatusCode == "pending" {
			first, ok := store.ParseTime(current.FirstFireTime)
			if rule.DurationSec <= 0 || (ok && now.Sub(first) >= time.Duration(rule.DurationSec)*time.Second) {
				current.StatusCode = "firing"
			}
			if err := touchIncident(ctx, db, tenantID, current, now, ""); err != nil {
				return err
			}
			if current.StatusCode == "firing" && current.NotifiedFlag != "Y" {
				return markFire(ctx, db, alerts, tenantID, rule, host, current, value, message, rule.ChannelID, silenced, now)
			}
			return nil
		}
		if err := touchIncident(ctx, db, tenantID, current, now, ""); err != nil {
			return err
		}
		if shouldEscalate(rule, current, now) {
			channelID := strings.TrimSpace(rule.EscalateChannelID)
			if channelID == "" {
				channelID = rule.ChannelID
			}
			sent, err := notify(ctx, alerts, tenantID, rule, host, current, value, message, "escalate", channelID, silenced, now)
			if err != nil || !sent {
				return err
			}
			current.EscalateFlag = "Y"
			return touchIncident(ctx, db, tenantID, current, now, "")
		}
		return nil
	}
	if !found {
		return nil
	}
	if current.StatusCode == "pending" {
		return deleteIncident(ctx, db, tenantID, current.IncidentID)
	}
	current.StatusCode = "resolved"
	current.CurrentValue = value
	current.MessageText = message
	if err := touchIncident(ctx, db, tenantID, current, now, store.FormatTime(now)); err != nil {
		return err
	}
	_, err = notify(ctx, alerts, tenantID, rule, host, current, value, message, "resolve", rule.ChannelID, silenced, now)
	return err
}

func markFire(ctx context.Context, db *dbx.DB, alerts *alert.Service, tenantID string, rule models.HostRule, host hostTarget, item Incident, value float64, message, channelID string, silenced bool, now time.Time) error {
	sent, err := notify(ctx, alerts, tenantID, rule, host, item, value, message, "fire", channelID, silenced, now)
	if err != nil || !sent {
		return err
	}
	item.NotifiedFlag = "Y"
	return touchIncident(ctx, db, tenantID, item, now, "")
}

func newIncident(rule models.HostRule, host hostTarget, value float64, status, message string) Incident {
	return Incident{
		RuleID: rule.RuleID, AgentID: host.AgentID, HostName: host.HostName,
		MetricCode: rule.MetricCode, LevelCode: rule.LevelCode, StatusCode: status,
		CurrentValue: value, ThresholdValue: rule.ThresholdValue, CompareOp: rule.CompareOp,
		MessageText: message, NotifiedFlag: "N", EscalateFlag: "N",
	}
}

func notify(ctx context.Context, alerts *alert.Service, tenantID string, rule models.HostRule, host hostTarget, item Incident, value float64, message, scene, channelID string, silenced bool, now time.Time) (bool, error) {
	if alerts == nil || silenced || strings.TrimSpace(channelID) == "" {
		return false, nil
	}
	extra, err := json.Marshal(map[string]string{
		"ruleName":   rule.RuleName,
		"level":      validate.LevelName(rule.LevelCode),
		"levelCode":  rule.LevelCode,
		"metric":     rule.MetricCode,
		"metricName": metricName(rule.MetricCode),
		"value":      formatNum(value),
		"threshold":  formatNum(rule.ThresholdValue),
		"operator":   opLabel(rule.CompareOp),
		"hostName":   host.HostName,
		"agentId":    host.AgentID,
		"firedAt":    item.FirstFireTime,
		"message":    message,
	})
	if err != nil {
		return false, err
	}
	if err := alerts.Submit(ctx, tenantID, alert.Notice{
		Level: rule.LevelCode, Type: "host", Scene: scene, Title: rule.RuleName,
		Content: message, ChannelID: channelID, Extra: string(extra),
	}, now); err != nil {
		return false, err
	}
	return true, nil
}

func shouldEscalate(rule models.HostRule, current Incident, now time.Time) bool {
	if rule.WaitMinutes <= 0 || current.EscalateFlag == "Y" {
		return false
	}
	if current.StatusCode != "firing" && current.StatusCode != "acked" {
		return false
	}
	first, ok := store.ParseTime(current.FirstFireTime)
	return ok && now.Sub(first) >= time.Duration(rule.WaitMinutes)*time.Minute
}

func listHosts(ctx context.Context, db *dbx.DB, tenantID string) ([]hostTarget, error) {
	rows, err := db.Query(ctx, `
		SELECT agentId, COALESCE(hostName, ''), lastSeenTime
		FROM FG_AGENT WHERE tenantId = @tenantId AND activeFlag = 'Y'
		ORDER BY agentId`, dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []hostTarget
	for rows.Next() {
		var item hostTarget
		if err := rows.Scan(&item.AgentID, &item.HostName, &item.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func metricValue(ctx context.Context, db *dbx.DB, tenantID string, host hostTarget, metric string, now time.Time) (float64, bool) {
	switch metric {
	case "cpu.usage":
		return latestFloat(ctx, db, `
			SELECT usagePercent FROM FG_HOST_CPU
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			ORDER BY observedTime DESC, pointId DESC LIMIT 1`, tenantID, host.AgentID)
	case "mem.usage":
		return latestFloat(ctx, db, `
			SELECT usedPercent FROM FG_HOST_MEM
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			ORDER BY observedTime DESC, pointId DESC LIMIT 1`, tenantID, host.AgentID)
	case "load.1":
		return latestFloat(ctx, db, `
			SELECT loadAvg1 FROM FG_HOST_CPU
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			ORDER BY observedTime DESC, pointId DESC LIMIT 1`, tenantID, host.AgentID)
	case "disk.usage":
		return latestFloat(ctx, db, `
			SELECT MAX(usedPercent) FROM FG_HOST_DISK
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			  AND batchId = (
			    SELECT batchId FROM FG_HOST_DISK
			    WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			    ORDER BY observedTime DESC, pointId DESC LIMIT 1
			  )`, tenantID, host.AgentID)
	case "disk.inode":
		return latestFloat(ctx, db, `
			SELECT MAX(inodeUsedPercent) FROM FG_HOST_DISK
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			  AND batchId = (
			    SELECT batchId FROM FG_HOST_DISK
			    WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			    ORDER BY observedTime DESC, pointId DESC LIMIT 1
			  )`, tenantID, host.AgentID)
	case "fd.usage":
		return latestFloat(ctx, db, `
			SELECT usedPercent FROM FG_HOST_FD
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
			ORDER BY observedTime DESC, pointId DESC LIMIT 1`, tenantID, host.AgentID)
	case "agent.offline":
		when, ok := store.TimeValue(host.LastSeen)
		if !ok {
			return 1e9, true
		}
		return now.Sub(when).Minutes(), true
	default:
		return 0, false
	}
}

func latestFloat(ctx context.Context, db *dbx.DB, query, tenantID, agentID string) (float64, bool) {
	var value any
	err := store.Scan(ctx, db, query, dbx.Args{"tenantId": tenantID, "agentId": agentID}, &value)
	if err != nil {
		return 0, false
	}
	return numberOf(value)
}
