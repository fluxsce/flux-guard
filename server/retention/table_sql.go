package retention

import (
	"fmt"
	"strings"
	"unicode"
)

// limitedDelete 生成限制条数的 DELETE。SQLite 用 rowid 子查询，MySQL 用 LIMIT。
func limitedDelete(driver, table, timeCol, extraWhere string, batch int) (string, error) {
	if !validIdent(table) || !validTimeCol(timeCol) {
		return "", fmt.Errorf("invalid table or time column")
	}
	if batch <= 0 {
		return "", fmt.Errorf("invalid batch size")
	}
	where := "tenantId = @tenantId AND " + timeCol + " < @before"
	if extra := strings.TrimSpace(extraWhere); extra != "" {
		where += " AND (" + extra + ")"
	}
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql", "mariadb":
		return fmt.Sprintf("DELETE FROM %s WHERE %s LIMIT %d", table, where, batch), nil
	default:
		// SQLite 对带 WHERE 的 DELETE 不支持 LIMIT，用 rowid 子查询。
		return fmt.Sprintf(
			"DELETE FROM %s WHERE rowid IN (SELECT rowid FROM %s WHERE %s LIMIT %d)",
			table, table, where, batch,
		), nil
	}
}

// validTimeCol 只接受字母、数字和下划线，避免把表达式拼进 SQL。
func validTimeCol(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// validIdent 表名须为标识符：字母或下划线开头，其余仅字母、数字、下划线。
func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
