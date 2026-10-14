package store

import (
	"context"
	"crypto/md5"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fluxguard/common/id"
	dbx "github.com/fluxsce/dbx/db"
)

const (
	statusSuccess       = "SUCCESS"
	statusAlreadyExists = "ALREADY_EXISTS"
	statusFailed        = "FAILED"
	historyScript       = "FG_SCRIPT_STATEMENT.sql"
)

// ensureStatementHistory 先执行驱动目录里的历史表脚本（不记账），后续才能按哈希跳过。
// DDL 只在 scripts/db/{driver}/FG_SCRIPT_STATEMENT.sql，不在 Go 里按驱动写死。
func ensureStatementHistory(ctx context.Context, conn *dbx.DB, dir string) error {
	body, err := os.ReadFile(filepath.Join(dir, historyScript))
	if err != nil {
		return fmt.Errorf("找不到语句历史脚本 %s: %w", historyScript, err)
	}
	for _, stmt := range statements(string(body)) {
		if _, err := conn.Exec(ctx, stmt, nil); err != nil {
			if isAlreadyPresent(err) || isIgnorableAlter(stmt, err) {
				continue
			}
			return fmt.Errorf("%s: %w", historyScript, err)
		}
	}
	return nil
}

func applyStatement(ctx context.Context, conn *dbx.DB, scriptFile, stmt string) error {
	hash := statementHash(stmt)
	stmtType := statementType(stmt)
	status, prevErr, err := statementStatus(ctx, conn, scriptFile, hash)
	if err != nil {
		return err
	}
	switch status {
	case statusSuccess, statusAlreadyExists:
		return nil
	case statusFailed:
		if isDropMissingIndex(stmt, fmt.Errorf("%s", prevErr)) || isAlreadyPresent(fmt.Errorf("%s", prevErr)) {
			return recordStatement(ctx, conn, scriptFile, hash, stmtType, stmt, statusAlreadyExists, 0, prevErr)
		}
	}

	started := time.Now()
	_, err = conn.Exec(ctx, stmt, nil)
	elapsed := time.Since(started)
	if err != nil {
		if isDropMissingIndex(stmt, err) || isAlreadyPresent(err) || isIgnorableAlter(stmt, err) {
			return recordStatement(ctx, conn, scriptFile, hash, stmtType, stmt, statusAlreadyExists, elapsed, err.Error())
		}
		_ = recordStatement(ctx, conn, scriptFile, hash, stmtType, stmt, statusFailed, elapsed, err.Error())
		return fmt.Errorf("%s: %w", scriptFile, err)
	}
	return recordStatement(ctx, conn, scriptFile, hash, stmtType, stmt, statusSuccess, elapsed, "")
}

func statementStatus(ctx context.Context, conn *dbx.DB, script, hash string) (status, errMsg string, err error) {
	// 唯一键保证最多一行，不依赖 LIMIT（Oracle 等方言另行分页）。
	err = Scan(ctx, conn, `
		SELECT executionStatus, COALESCE(errorMessage, '') FROM FG_SCRIPT_STATEMENT
		WHERE tenantId = @tenantId AND scriptName = @scriptName
		  AND statementHash = @statementHash AND databaseDriver = @databaseDriver`,
		dbx.Args{
			"tenantId": "default", "scriptName": script,
			"statementHash": hash, "databaseDriver": scriptName(conn.Driver()),
		}, &status, &errMsg)
	if err != nil {
		if isNoRows(err) {
			return "", "", nil
		}
		return "", "", err
	}
	return status, errMsg, nil
}

func recordStatement(ctx context.Context, conn *dbx.DB, script, hash, stmtType, content, status string, elapsed time.Duration, errMsg string) error {
	now := FormatTime(time.Now().UTC())
	driver := scriptName(conn.Driver())
	var existing string
	err := Scan(ctx, conn, `
		SELECT statementId FROM FG_SCRIPT_STATEMENT
		WHERE tenantId = @tenantId AND scriptName = @scriptName
		  AND statementHash = @statementHash AND databaseDriver = @databaseDriver`,
		dbx.Args{
			"tenantId": "default", "scriptName": script,
			"statementHash": hash, "databaseDriver": driver,
		}, &existing)
	if err != nil && !isNoRows(err) {
		return err
	}
	if existing != "" {
		_, err = conn.Exec(ctx, `
			UPDATE FG_SCRIPT_STATEMENT SET
			  executionStatus = @executionStatus, executionTime = @executionTime,
			  executionDuration = @executionDuration, errorMessage = @errorMessage,
			  editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND statementId = @statementId`,
			dbx.Args{
				"executionStatus": status, "executionTime": now,
				"executionDuration": elapsed.Milliseconds(), "errorMessage": nullIfEmpty(errMsg),
				"editTime": now, "tenantId": "default", "statementId": existing,
			})
		return err
	}
	_, err = conn.Exec(ctx, `
		INSERT INTO FG_SCRIPT_STATEMENT (
		  statementId, tenantId, scriptName, statementHash, statementType, statementContent,
		  databaseDriver, executionStatus, executionTime, executionDuration, errorMessage,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @statementId, @tenantId, @scriptName, @statementHash, @statementType, @statementContent,
		  @databaseDriver, @executionStatus, @executionTime, @executionDuration, @errorMessage,
		  'system', 'system', 'INIT', 1, 'Y'
		)`, dbx.Args{
		"statementId": id.New(), "tenantId": "default", "scriptName": script,
		"statementHash": hash, "statementType": stmtType, "statementContent": content,
		"databaseDriver": driver, "executionStatus": status, "executionTime": now,
		"executionDuration": elapsed.Milliseconds(), "errorMessage": nullIfEmpty(errMsg),
	})
	return err
}

func statementHash(stmt string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(stmt, "\r\n", "\n"))
	sum := md5.Sum([]byte(normalized))
	return fmt.Sprintf("%x", sum)
}

func statementType(stmt string) string {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	switch {
	case strings.HasPrefix(upper, "CREATE TABLE"):
		return "CREATE_TABLE"
	case strings.HasPrefix(upper, "CREATE UNIQUE INDEX"):
		return "CREATE_UNIQUE_INDEX"
	case strings.HasPrefix(upper, "CREATE INDEX"):
		return "CREATE_INDEX"
	case strings.HasPrefix(upper, "ALTER"):
		return "ALTER"
	case strings.HasPrefix(upper, "UPDATE"):
		return "UPDATE"
	case strings.HasPrefix(upper, "INSERT"):
		return "INSERT"
	case strings.HasPrefix(upper, "DROP"):
		return "DROP"
	default:
		return "OTHER"
	}
}

func isAlreadyPresent(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"already exists", "duplicate entry", "duplicate column", "duplicate key",
		"unique constraint failed", "unique constraint violated",
		"constraint failed", "primary key",
		"ora-00955", "ora-01430", "ora-01408", "ora-00001",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func isDropMissingIndex(stmt string, err error) bool {
	if err == nil || !isDropIndexStatement(stmt) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "check that column/key exists") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "no such index")
}

func isDropIndexStatement(stmt string) bool {
	s := strings.ToLower(strings.Join(strings.Fields(stmt), " "))
	return strings.Contains(s, " drop index ") || strings.HasPrefix(s, "drop index ") ||
		strings.Contains(s, " drop key ") || strings.HasPrefix(s, "drop key ")
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
