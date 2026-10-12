// Package panel 读取组件监控页签。行在 FG_COMPONENT_PANEL，启动时不另写一份。
package panel

import (
	"context"
	"strings"

	"fluxguard/server/component/row"
	dbx "github.com/fluxsce/dbx/db"
)

// Panel 是详情里的一页。概览收走其它页签没有点名的分节。
type Panel struct {
	KindCode  string
	PanelCode string
	PanelName string
	SortOrder int
	Terminal  bool
	List      bool
	Metrics   []string
	Sections  []string
}

// Of 返回一种组件在该租户下的页签，按库里的排序。
func Of(ctx context.Context, db *dbx.DB, tenantID, kind string) ([]Panel, error) {
	if db == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(kind) == "" {
		return nil, nil
	}
	rows, err := db.Query(ctx, `
		SELECT panelCode, panelName, sortOrder, terminalFlag, listFlag,
		       COALESCE(metricText, ''), COALESCE(sectionText, '')
		FROM FG_COMPONENT_PANEL
		WHERE tenantId = @tenantId AND kindCode = @kindCode AND activeFlag = 'Y' AND statusFlag = 'Y'
		ORDER BY sortOrder, panelCode`,
		dbx.Args{"tenantId": tenantID, "kindCode": kind})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Panel
	for rows.Next() {
		var code, name, terminal, list, metrics, sections any
		var sort any
		if err := rows.Scan(&code, &name, &sort, &terminal, &list, &metrics, &sections); err != nil {
			return nil, err
		}
		out = append(out, Panel{
			KindCode: kind, PanelCode: row.Text(code), PanelName: row.Text(name),
			SortOrder: int(row.Float(sort)), Terminal: row.Text(terminal) == "Y", List: row.Text(list) == "Y",
			Metrics: split(row.Text(metrics)), Sections: split(row.Text(sections)),
		})
	}
	return out, rows.Err()
}

// ListMetrics 是实例列表上要直接显示的当前值。列名必须在该种类的当前值表里。
func ListMetrics(ctx context.Context, db *dbx.DB, tenantID, kind string) ([]string, error) {
	items, err := Of(ctx, db, tenantID, kind)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]struct{}{}
	for _, item := range items {
		if !item.List {
			continue
		}
		for _, code := range item.Metrics {
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			out = append(out, code)
		}
	}
	return out, nil
}

func split(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
