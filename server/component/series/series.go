// Package series 按种类读取只插入的采集表。列名来自目录，不接受请求里的任意列。
package series

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"fluxguard/server/component/models"
	"fluxguard/server/component/row"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

var ident = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// Span 是曲线的时间窗。起止都为空时仍按条数取最近一批点。
type Span struct {
	From time.Time
	To   time.Time
}

// Read 返回时间窗内的点，时间从早到晚。未给时间窗时最多 500 条，给出时间窗时最多 4000 条。
func Read(ctx context.Context, db *dbx.DB, table, tenantID, componentID string, cols []string, limit int, span Span) ([]models.Point, error) {
	if db == nil || !ident.MatchString(table) {
		return nil, fmt.Errorf("采集表无效")
	}
	for _, col := range cols {
		if !ident.MatchString(col) {
			return nil, fmt.Errorf("指标列无效")
		}
	}
	ranged := !span.From.IsZero() || !span.To.IsZero()
	if ranged {
		if limit < 1 || limit > 4000 {
			limit = 4000
		}
	} else if limit < 1 || limit > 500 {
		limit = 120
	}
	// 曲线不带 extProperty。慢日志这类 JSON 只在最新一点上取。
	// 条件与排序对齐 IDX_*_TIME (tenantId, componentId, observedTime)，采集行只插入，不再为 activeFlag 另建索引。
	where := "WHERE tenantId = @tenantId AND componentId = @componentId"
	args := dbx.Args{"tenantId": tenantID, "componentId": componentID}
	if !span.From.IsZero() {
		where += " AND observedTime >= @fromTime"
		args["fromTime"] = store.FormatTime(span.From)
	}
	if !span.To.IsZero() {
		where += " AND observedTime <= @toTime"
		args["toTime"] = store.FormatTime(span.To)
	}
	query := fmt.Sprintf(`
		SELECT upFlag, COALESCE(versionText, ''), COALESCE(roleName, ''), COALESCE(errorText, ''),
		       COALESCE(summaryText, ''), observedTime, %s
		FROM %s
		%s
		ORDER BY observedTime DESC
		LIMIT %d`, selectCols(cols), table, where, limit)
	rows, err := db.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Point
	for rows.Next() {
		vals := make([]any, 6+len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		point := models.Point{
			UpFlag:       row.Text(deref(vals[0])),
			VersionText:  row.Text(deref(vals[1])),
			RoleName:     row.Text(deref(vals[2])),
			ErrorText:    row.Text(deref(vals[3])),
			SummaryText:  row.Text(deref(vals[4])),
			ObservedTime: row.Text(deref(vals[5])),
			Values:       map[string]float64{},
		}
		for i, col := range cols {
			point.Values[col] = row.Float(deref(vals[6+i]))
		}
		out = append(out, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) > 0 {
		raw, err := latestDetail(ctx, db, table, tenantID, componentID)
		if err != nil {
			return nil, err
		}
		if raw = strings.TrimSpace(raw); json.Valid([]byte(raw)) && raw != "{}" {
			out[0].Detail = json.RawMessage(raw)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if out == nil {
		out = []models.Point{}
	}
	return out, nil
}

func latestDetail(ctx context.Context, db *dbx.DB, table, tenantID, componentID string) (string, error) {
	var raw any
	err := store.Scan(ctx, db, fmt.Sprintf(`
		SELECT COALESCE(extProperty, '')
		FROM %s
		WHERE tenantId = @tenantId AND componentId = @componentId
		ORDER BY observedTime DESC
		LIMIT 1`, table), dbx.Args{"tenantId": tenantID, "componentId": componentID}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Text(raw), nil
}

func selectCols(cols []string) string {
	if len(cols) == 0 {
		return "0"
	}
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = col
	}
	return strings.Join(parts, ", ")
}

func deref(value any) any {
	ptr, ok := value.(*any)
	if !ok || ptr == nil {
		return value
	}
	return *ptr
}
