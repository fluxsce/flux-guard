package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fluxguard/common/id"
	dbx "github.com/fluxsce/dbx/db"
)

const seedScript = "seed"

// initialize 写入开发种子。已有成功记录后不再执行。失败回滚，不留下失败行，下次启动会再试。
func initialize(ctx context.Context, conn *dbx.DB, dir string, files []string) error {
	var done int
	if err := Scan(ctx, conn, `
		SELECT count(*) FROM FG_SCRIPT_INIT
		WHERE tenantId = @tenantId AND scriptName = @scriptName AND executionStatus = 'SUCCESS'`,
		dbx.Args{"tenantId": "default", "scriptName": seedScript}, &done); err != nil {
		return err
	}
	if done > 0 {
		return nil
	}
	return conn.Tx(ctx, func(tx *dbx.DB) error {
		started := time.Now()
		count, err := seed(ctx, tx, dir, files)
		if err != nil {
			return err
		}
		return markInitialized(ctx, tx, count, time.Since(started))
	})
}

func seed(ctx context.Context, conn *dbx.DB, dir string, files []string) (int, error) {
	count := 0
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return 0, err
		}
		for _, stmt := range statements(string(body)) {
			if isSchema(stmt) {
				continue
			}
			if _, err := conn.Exec(ctx, stmt, nil); err != nil {
				return 0, fmt.Errorf("%s: %w", name, err)
			}
			count++
		}
	}
	return count, nil
}

func markInitialized(ctx context.Context, conn *dbx.DB, statements int, elapsed time.Duration) error {
	now := FormatTime(time.Now().UTC())
	_, err := conn.Exec(ctx, `
		INSERT INTO FG_SCRIPT_INIT (
		  initId, tenantId, scriptName, scriptPath, scriptVersion, databaseDriver,
		  executionStatus, executionTime, executionDuration, statementsExecuted,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @initId, @tenantId, @scriptName, @scriptPath, @scriptVersion, @databaseDriver,
		  'SUCCESS', @executionTime, @executionDuration, @statementsExecuted,
		  'system', 'system', 'INIT', 1, 'Y', @noteText
		)`, dbx.Args{
		"initId": id.New(), "tenantId": "default", "scriptName": seedScript,
		"scriptPath": "scripts/db/" + conn.Driver(), "scriptVersion": "1", "databaseDriver": conn.Driver(),
		"executionTime": now, "executionDuration": elapsed.Milliseconds(), "statementsExecuted": statements,
		"noteText": "开发种子只执行一次",
	})
	return err
}
