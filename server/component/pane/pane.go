// Package pane 保存每种组件的当前值。每个连接一行，列表只读这张表。
package pane

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/panel"
	"fluxguard/server/component/row"
	dbx "github.com/fluxsce/dbx/db"
)

// MaxColumns 是当前值表的列数上限。只放列表要看的指标，曲线仍在采集表。
const MaxColumns = 32

var (
	ident = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	idOK  = regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`)
)

// Touch 用这次采集里的列表指标覆盖该连接的当前行。
func Touch(ctx context.Context, db *dbx.DB, kind string, args dbx.Args) error {
	if db == nil {
		return nil
	}
	spec, ok := catalog.Get(kind)
	if !ok || spec.PaneTable == "" {
		return nil
	}
	cols := panel.ListMetrics(kind)
	if len(cols) == 0 {
		return nil
	}
	if !ident.MatchString(spec.PaneTable) {
		return fmt.Errorf("当前值表无效")
	}
	for _, col := range cols {
		if !ident.MatchString(col) || len(col) > MaxColumns {
			return fmt.Errorf("指标列无效")
		}
		if _, ok := args[col]; !ok {
			return fmt.Errorf("当前值缺少 %s", col)
		}
	}
	_, err := db.Exec(ctx, upsertSQL(db.Driver(), spec.PaneTable, cols), args)
	return err
}

// Latest 按连接读取当前值。只查当前值表，不扫采集历史。
func Latest(ctx context.Context, db *dbx.DB, table, tenantID string, ids, cols []string) (map[string]map[string]float64, error) {
	out := map[string]map[string]float64{}
	if db == nil || len(ids) == 0 || len(cols) == 0 || !ident.MatchString(table) {
		return out, nil
	}
	for _, col := range cols {
		if !ident.MatchString(col) {
			return nil, fmt.Errorf("指标列无效")
		}
	}
	args := dbx.Args{"tenantId": tenantID}
	holders := make([]string, 0, len(ids))
	for i, id := range ids {
		if !idOK.MatchString(id) {
			continue
		}
		key := fmt.Sprintf("id%d", i)
		args[key] = id
		holders = append(holders, "@"+key)
	}
	if len(holders) == 0 {
		return out, nil
	}
	query := fmt.Sprintf(`
		SELECT componentId, %s
		FROM %s
		WHERE tenantId = @tenantId AND componentId IN (%s)`,
		strings.Join(cols, ", "), table, strings.Join(holders, ", "))
	rows, err := db.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		vals := make([]any, 1+len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		id := row.Text(deref(vals[0]))
		if id == "" {
			continue
		}
		point := map[string]float64{}
		for i, col := range cols {
			point[col] = row.Float(deref(vals[1+i]))
		}
		out[id] = point
	}
	return out, rows.Err()
}

func upsertSQL(driver, table string, metrics []string) string {
	cols := append([]string{"tenantId", "componentId", "observedTime"}, metrics...)
	cols = append(cols, "addTime", "addWho", "editTime", "editWho", "oprSeqFlag", "currentVersion", "activeFlag")
	vals := make([]string, len(cols))
	for i, col := range cols {
		vals[i] = "@" + col
	}
	assign := append([]string{"observedTime"}, metrics...)
	assign = append(assign, "editTime", "editWho", "oprSeqFlag")
	if strings.Contains(strings.ToLower(driver), "mysql") {
		sets := make([]string, 0, len(assign)+1)
		for _, col := range assign {
			sets = append(sets, col+" = VALUES("+col+")")
		}
		sets = append(sets, "currentVersion = currentVersion + 1")
		return "INSERT INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(vals, ", ") + ") ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", ")
	}
	sets := make([]string, 0, len(assign)+1)
	for _, col := range assign {
		sets = append(sets, col+" = excluded."+col)
	}
	sets = append(sets, "currentVersion = "+table+".currentVersion + 1")
	return "INSERT INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(vals, ", ") + ") ON CONFLICT(tenantId, componentId) DO UPDATE SET " + strings.Join(sets, ", ")
}

func deref(value any) any {
	ptr, ok := value.(*any)
	if !ok || ptr == nil {
		return value
	}
	return *ptr
}
