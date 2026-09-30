// Package tenant 为租户级任务列举有效租户。调度核不查询业务表。
package tenant

import (
	"context"
	"strings"

	dbx "github.com/fluxsce/dbx/db"
)

// List 读有效租户。没有行时返回 fallback。
func List(ctx context.Context, db *dbx.DB, fallback string) ([]string, error) {
	if db == nil {
		return one(fallback), nil
	}
	rows, err := db.Query(ctx, `SELECT tenantId FROM FG_TENANT WHERE activeFlag = 'Y'`, nil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id != "" {
			out = append(out, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return one(fallback), nil
	}
	return out, nil
}

// Lister 供调度器使用。
func Lister(db *dbx.DB, fallback string) func(context.Context) ([]string, error) {
	return func(ctx context.Context) ([]string, error) {
		return List(ctx, db, fallback)
	}
}

func one(fallback string) []string {
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		fallback = "default"
	}
	return []string{fallback}
}
