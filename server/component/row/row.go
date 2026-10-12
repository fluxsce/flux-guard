// Package row 给组件表补上统一的审计列，并收束库里读出的数字和文本。
package row

import (
	"strconv"
	"time"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// Stamp 是插入一行时的审计字段。
func Stamp(who, note string) dbx.Args {
	now := store.FormatTime(time.Now().UTC())
	if who == "" {
		who = "system"
	}
	return dbx.Args{
		"addTime": now, "addWho": who, "editTime": now, "editWho": who,
		"oprSeqFlag": id.New(), "currentVersion": 1, "activeFlag": "Y",
		"noteText": note,
	}
}

// Touch 是更新一行时的审计字段。
func Touch(who string) dbx.Args {
	if who == "" {
		who = "system"
	}
	return dbx.Args{
		"editTime": store.FormatTime(time.Now().UTC()), "editWho": who, "oprSeqFlag": id.New(),
	}
}

// Flag 把是否存活写成 Y/N。
func Flag(up bool) string {
	if up {
		return "Y"
	}
	return "N"
}

// Cut 按字符数截断，避免超出 VARCHAR。
func Cut(value string, n int) string {
	if n <= 0 || utf8.RuneCountInString(value) <= n {
		return value
	}
	runes := []rune(value)
	return string(runes[:n])
}

// Text 把驱动返回值收成字符串。
func Text(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		when, ok := store.TimeValue(typed)
		if !ok {
			return ""
		}
		return store.FormatTime(when)
	default:
		if when, ok := store.TimeValue(value); ok {
			return store.FormatTime(when)
		}
		return ""
	}
}

// Float 把驱动返回的整数、小数或数字文本收成 float64。
func Float(value any) float64 {
	switch typed := value.(type) {
	case nil:
		return 0
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case float64:
		return typed
	case float32:
		return float64(typed)
	case []byte:
		out, _ := strconv.ParseFloat(string(typed), 64)
		return out
	case string:
		out, _ := strconv.ParseFloat(typed, 64)
		return out
	default:
		out, _ := strconv.ParseFloat(Text(typed), 64)
		return out
	}
}

// Int 把数字收成 int64。
func Int(value any) int64 {
	return int64(Float(value))
}
