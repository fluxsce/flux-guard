package store

import "time"

const sqlTime = "2006-01-02 15:04:05"

// FormatTime 把时间写成 UTC 的「年-月-日 时:分:秒」。SQLite 文本和 MySQL DATETIME 都能存。
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(sqlTime)
}

// ParseTime 读取本包写入的时间，也接受 SQLite 的 datetime 文本。
func ParseTime(text string) (time.Time, bool) {
	if text == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return t.UTC(), true
	}
	if t, err := time.ParseInLocation(sqlTime, text, time.UTC); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// TimeValue 把驱动返回的时间收成 UTC。SQLite 给文本，MySQL 给 time.Time。
func TimeValue(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return time.Time{}, false
		}
		return t.UTC(), true
	case string:
		return ParseTime(t)
	case []byte:
		return ParseTime(string(t))
	default:
		return time.Time{}, false
	}
}
