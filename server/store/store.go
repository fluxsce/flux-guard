// Package store 用 dbx 打开中央库并执行 scripts/db 下的脚本。
// 驱动名对应 scripts/db 下的目录。当前登记了 sqlite 和 mysql。
// 脚本按分号拆成语句，内容 MD5 记入 FG_SCRIPT_STATEMENT；已成功的下次启动跳过。
// 脚本顺序读各目录的 init.sql。
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbx "github.com/fluxsce/dbx/db"
)

// Open 按驱动打开库并建表。driver 为空时使用 sqlite。
func Open(ctx context.Context, driver, dsn string) (*dbx.DB, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		driver = "sqlite"
	}
	script := scriptName(driver)
	if script == "sqlite" {
		if dir := filepath.Dir(dsn); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}
		if !strings.Contains(dsn, "?") {
			dsn += "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
		}
	}
	pool := dbx.Config{
		Driver:          driver,
		DSN:             dsn,
		MaxOpenConns:    20,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
	if script == "sqlite" {
		pool.MaxOpenConns = 1
		pool.MaxIdleConns = 1
	}
	conn, err := dbx.Open(ctx, pool)
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, conn, script); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// OpenTemp 打开临时目录中的 SQLite 文件。cleanup 关闭连接并删除该目录。测试使用。
func OpenTemp(ctx context.Context) (*dbx.DB, func(), error) {
	dir, err := os.MkdirTemp("", "fluxguard-")
	if err != nil {
		return nil, nil, err
	}
	conn, err := Open(ctx, "sqlite", filepath.Join(dir, "flux.db"))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, nil, err
	}
	cleanup := func() {
		_ = conn.Close()
		_ = os.RemoveAll(dir)
	}
	return conn, cleanup, nil
}

// Scan 读取一行。无行时返回 sql.ErrNoRows。
func Scan(ctx context.Context, conn *dbx.DB, query string, args dbx.Args, dest ...any) error {
	row, err := conn.QueryRow(ctx, query, args)
	if err != nil {
		return err
	}
	return row.Scan(dest...)
}

func scriptName(driver string) string {
	switch strings.ToLower(driver) {
	case "sqlite", "sqlite3":
		return "sqlite"
	case "mysql", "mariadb":
		return "mysql"
	default:
		return strings.ToLower(driver)
	}
}

func migrate(ctx context.Context, conn *dbx.DB, script string) error {
	dir, err := schemaDir(script)
	if err != nil {
		return err
	}
	files, err := scriptFiles(dir)
	if err != nil {
		return err
	}
	if err := ensureStatementHistory(ctx, conn, dir); err != nil {
		return err
	}
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		for _, stmt := range statements(string(body)) {
			if err := applyStatement(ctx, conn, name, stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func schemaDir(script string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "scripts", "db", script)
		if _, err := os.Stat(filepath.Join(candidate, "init.sql")); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("找不到 scripts/db/%s，请在仓库根目录启动", script)
}

// scriptFiles 按 init.sql 里的 .read / source 顺序取出脚本文件名。
func scriptFiles(dir string) ([]string, error) {
	body, err := os.ReadFile(filepath.Join(dir, "init.sql"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		line = strings.TrimSuffix(line, ";")
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case ".read", "source":
			names = append(names, fields[1])
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s 没有列出脚本", filepath.Join(dir, "init.sql"))
	}
	return names, nil
}

// isIgnorableAlter 重复执行时忽略「列已存在」，以及旧采集表已经改名或不存在。
func isIgnorableAlter(stmt string, err error) bool {
	if err == nil || !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "ALTER") {
		return false
	}
	msg := strings.ToLower(err.Error())
	upper := strings.ToUpper(stmt)
	if strings.Contains(upper, " DROP INDEX ") || strings.Contains(upper, " DROP KEY ") {
		return strings.Contains(msg, "check that column/key exists") ||
			strings.Contains(msg, "doesn't exist") ||
			strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "no such index")
	}
	if strings.Contains(upper, " RENAME ") {
		return strings.Contains(msg, "no such table") ||
			strings.Contains(msg, "doesn't exist") ||
			strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "already exists") ||
			strings.Contains(msg, "already another table")
	}
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "duplicate key name") ||
		strings.Contains(msg, "already exists")
}

// statements 按分号拆分脚本；字符串字面量内的分号不拆。
func statements(text string) []string {
	var out []string
	var cur strings.Builder
	inString := false
	var quote byte

	flush := func() {
		stmt := strings.TrimSpace(cur.String())
		cur.Reset()
		if stmt == "" {
			return
		}
		out = append(out, stmt)
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inString && (trimmed == "" || strings.HasPrefix(trimmed, "--")) {
			continue
		}
		for i := 0; i < len(line); i++ {
			ch := line[i]
			if !inString {
				if ch == '\'' || ch == '"' {
					inString = true
					quote = ch
				} else if ch == ';' {
					flush()
					continue
				}
				cur.WriteByte(ch)
				continue
			}
			cur.WriteByte(ch)
			if ch == quote {
				if quote == '\'' && i+1 < len(line) && line[i+1] == '\'' {
					cur.WriteByte(line[i+1])
					i++
					continue
				}
				inString = false
				quote = 0
			}
		}
		if inString || cur.Len() > 0 {
			cur.WriteByte('\n')
		}
	}
	flush()
	return out
}
