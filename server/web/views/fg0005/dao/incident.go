package dao

import (
	"context"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/alert/validate"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// Incident 是一条主机未关闭告警。持续和升级记在这里，发送交给预警日志。
type Incident struct {
	IncidentID     string
	RuleID         string
	AgentID        string
	HostName       string
	MetricCode     string
	LevelCode      string
	StatusCode     string
	CurrentValue   float64
	ThresholdValue float64
	CompareOp      string
	FirstFireTime  string
	LastFireTime   string
	MessageText    string
	NotifiedFlag   string
	EscalateFlag   string
}

const incidentSelect = `
	SELECT incidentId, ruleId, agentId, COALESCE(hostName, ''), metricCode, levelCode, statusCode,
	       currentValue, thresholdValue, compareOp, firstFireTime, lastFireTime,
	       COALESCE(messageText, ''), notifiedFlag, escalateFlag
	FROM FG_HOST_INCIDENT`

// ListIncidents 返回租户下的主机告警记录，供判定和测试核对状态。
func ListIncidents(ctx context.Context, db *dbx.DB, tenantID string) ([]Incident, error) {
	rows, err := db.Query(ctx, incidentSelect+` WHERE tenantId = @tenantId AND activeFlag = 'Y' ORDER BY firstFireTime`,
		dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Incident
	for rows.Next() {
		item, err := scanIncident(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func openIncident(ctx context.Context, db *dbx.DB, tenantID, ruleID, agentID string) (Incident, bool, error) {
	query, args, err := store.PageQuery(db, incidentSelect+`
		WHERE tenantId = @tenantId AND ruleId = @ruleId AND agentId = @agentId AND activeFlag = 'Y'
		  AND statusCode IN ('pending', 'firing', 'acked')
		ORDER BY firstFireTime DESC`, dbx.Args{"tenantId": tenantID, "ruleId": ruleID, "agentId": agentID}, 1)
	if err != nil {
		return Incident{}, false, err
	}
	rows, err := db.Query(ctx, query, args)
	if err != nil {
		return Incident{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Incident{}, false, rows.Err()
	}
	item, err := scanIncident(rows.Scan)
	if err != nil {
		return Incident{}, false, err
	}
	return item, true, nil
}

func insertIncident(ctx context.Context, db *dbx.DB, tenantID string, item Incident, now time.Time) (Incident, error) {
	when := store.FormatTime(now)
	item.IncidentID = id.New()
	if item.FirstFireTime == "" {
		item.FirstFireTime = when
	}
	item.LastFireTime = when
	if item.NotifiedFlag == "" {
		item.NotifiedFlag = "N"
	}
	if item.EscalateFlag == "" {
		item.EscalateFlag = "N"
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_HOST_INCIDENT (
		  tenantId, incidentId, ruleId, agentId, hostName, metricCode, levelCode, statusCode,
		  currentValue, thresholdValue, compareOp, firstFireTime, lastFireTime, messageText,
		  notifiedFlag, escalateFlag, addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @tenantId, @incidentId, @ruleId, @agentId, @hostName, @metricCode, @levelCode, @statusCode,
		  @currentValue, @thresholdValue, @compareOp, @firstFireTime, @lastFireTime, @messageText,
		  @notifiedFlag, @escalateFlag, @addTime, 'system', @editTime, 'system', @oprSeqFlag, 1, 'Y'
		)`, dbx.Args{
		"tenantId": tenantID, "incidentId": item.IncidentID, "ruleId": item.RuleID, "agentId": item.AgentID,
		"hostName": item.HostName, "metricCode": item.MetricCode, "levelCode": item.LevelCode, "statusCode": item.StatusCode,
		"currentValue": item.CurrentValue, "thresholdValue": item.ThresholdValue, "compareOp": item.CompareOp,
		"firstFireTime": item.FirstFireTime, "lastFireTime": item.LastFireTime, "messageText": item.MessageText,
		"notifiedFlag": item.NotifiedFlag, "escalateFlag": item.EscalateFlag,
		"addTime": when, "editTime": when, "oprSeqFlag": id.New(),
	})
	return item, err
}

func touchIncident(ctx context.Context, db *dbx.DB, tenantID string, item Incident, now time.Time, resolveTime string) error {
	when := store.FormatTime(now)
	_, err := db.Exec(ctx, `
		UPDATE FG_HOST_INCIDENT SET
		  statusCode = @statusCode, currentValue = @currentValue, lastFireTime = @lastFireTime,
		  messageText = @messageText, resolveTime = @resolveTime,
		  notifiedFlag = @notifiedFlag, escalateFlag = @escalateFlag,
		  editTime = @editTime, editWho = 'system', oprSeqFlag = @oprSeqFlag, currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND incidentId = @incidentId`, dbx.Args{
		"statusCode": item.StatusCode, "currentValue": item.CurrentValue, "lastFireTime": when,
		"messageText": item.MessageText, "resolveTime": emptyText(resolveTime),
		"notifiedFlag": item.NotifiedFlag, "escalateFlag": item.EscalateFlag,
		"editTime": when, "oprSeqFlag": id.New(), "tenantId": tenantID, "incidentId": item.IncidentID,
	})
	return err
}

func deleteIncident(ctx context.Context, db *dbx.DB, tenantID, incidentID string) error {
	_, err := db.Exec(ctx, `DELETE FROM FG_HOST_INCIDENT WHERE tenantId = @tenantId AND incidentId = @incidentId`,
		dbx.Args{"tenantId": tenantID, "incidentId": incidentID})
	return err
}

func scanIncident(scan func(dest ...any) error) (Incident, error) {
	var current, threshold, firstFire, lastFire any
	var item Incident
	err := scan(&item.IncidentID, &item.RuleID, &item.AgentID, &item.HostName, &item.MetricCode, &item.LevelCode, &item.StatusCode,
		&current, &threshold, &item.CompareOp, &firstFire, &lastFire, &item.MessageText, &item.NotifiedFlag, &item.EscalateFlag)
	if err != nil {
		return Incident{}, err
	}
	item.CurrentValue, _ = validate.AsFloat(current)
	item.ThresholdValue, _ = validate.AsFloat(threshold)
	item.FirstFireTime = validate.AsText(firstFire)
	item.LastFireTime = validate.AsText(lastFire)
	return item, nil
}

func emptyText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
