package catalog

import (
	"context"

	"fluxguard/server/component/row"
	dbx "github.com/fluxsce/dbx/db"
)

// Ensure 把目录写入 FG_COMPONENT_KIND。已有的种类不覆盖，避免改掉库里的备注。
func Ensure(ctx context.Context, db *dbx.DB) error {
	if db == nil {
		return nil
	}
	tenants, err := tenantIDs(ctx, db)
	if err != nil {
		return err
	}
	for _, tenantID := range tenants {
		for _, kind := range All() {
			var n int
			if err := scanCount(ctx, db, tenantID, kind.KindCode, &n); err != nil {
				return err
			}
			if n > 0 {
				continue
			}
			args := dbx.Args{
				"tenantId": tenantID, "kindCode": kind.KindCode, "kindName": kind.KindName,
				"defaultPort": kind.DefaultPort, "metricTable": kind.MetricTable, "sortOrder": kind.SortOrder,
				"statusFlag": "Y", "noteText": kind.NoteText,
			}
			for key, value := range row.Stamp("system", kind.NoteText) {
				args[key] = value
			}
			if _, err := db.Exec(ctx, `
				INSERT INTO FG_COMPONENT_KIND (
				  tenantId, kindCode, kindName, defaultPort, metricTable, sortOrder, statusFlag,
				  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
				) VALUES (
				  @tenantId, @kindCode, @kindName, @defaultPort, @metricTable, @sortOrder, @statusFlag,
				  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
				)`, args); err != nil {
				return err
			}
		}
	}
	return nil
}

func tenantIDs(ctx context.Context, db *dbx.DB) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT tenantId FROM FG_TENANT WHERE activeFlag = 'Y'`, nil)
	if err != nil {
		return []string{"default"}, nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		out = []string{"default"}
	}
	return out, rows.Err()
}

func scanCount(ctx context.Context, db *dbx.DB, tenantID, kindCode string, dest *int) error {
	row, err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM FG_COMPONENT_KIND
		WHERE tenantId = @tenantId AND kindCode = @kindCode`,
		dbx.Args{"tenantId": tenantID, "kindCode": kindCode})
	if err != nil {
		return err
	}
	return row.Scan(dest)
}
