package store

import (
	"strings"

	dbx "github.com/fluxsce/dbx/db"
)

// LikeClause 在 raw 非空时追加不区分大小写的包含条件。
// column 必须是调用方写死的列，不能来自请求。
func LikeClause(args dbx.Args, column, key, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.NewReplacer("%", "", "_", "").Replace(raw)
	args[key] = "%" + strings.ToLower(raw) + "%"
	return " AND LOWER(" + column + ") LIKE @" + key
}

// EqClause 在 raw 非空时追加等值条件。column 必须是调用方写死的列。
func EqClause(args dbx.Args, column, key, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	args[key] = raw
	return " AND " + column + " = @" + key
}

// PageQuery 按当前引擎给已有 ORDER BY 的查询加上一页，并写入 limit / offset。
func PageQuery(db *dbx.DB, query string, args dbx.Args, size int) (string, dbx.Args, error) {
	page := dbx.Page{Page: 1, PageSize: size}
	paged, err := db.PageSQL(query, page)
	if err != nil {
		return "", nil, err
	}
	return paged, dbx.BindPage(args, page), nil
}
