package setting

import (
	"context"
	"encoding/json"

	dbx "github.com/fluxsce/dbx/db"
)

// Load 把已落库的访问策略、环境变量和上报令牌读进缓存。没有行时保持默认值。
// 任务策略（数据归档等）由 server/job.Load 加载，不在此包。
func Load(ctx context.Context, db *dbx.DB) error {
	rows, err := db.Query(ctx, `
		SELECT tenantId, groupCode, content
		FROM FG_SYS_SETTING
		WHERE activeFlag = 'Y' AND groupCode IN ('webTimeout', 'envVars', 'ingestToken')`, nil)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tenantID, groupCode, content string
		if err := rows.Scan(&tenantID, &groupCode, &content); err != nil {
			return err
		}
		switch groupCode {
		case GroupWebTimeout:
			Put(tenantID, Parse(content))
		case GroupEnvVars:
			PutEnvVars(tenantID, ParseEnvVars(content))
		case GroupIngestToken:
			PutIngest(tenantID, ParseIngest(content))
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return installDevIngest(ctx, db)
}

// Encode 把访问策略写成入库 JSON。
func Encode(v WebTimeout) (string, error) {
	raw, err := json.Marshal(merge(v))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
