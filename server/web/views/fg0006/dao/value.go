package dao

import (
	"strconv"
	"time"

	"fluxguard/server/store"
)

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	case []byte:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		i, err := strconv.ParseInt(string(n), 10, 64)
		if err != nil {
			f, _ := strconv.ParseFloat(string(n), 64)
			return int64(f)
		}
		return i
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			f, _ := strconv.ParseFloat(n, 64)
			return int64(f)
		}
		return i
	default:
		return 0
	}
}

func timeText(v any) string {
	if t, ok := store.TimeValue(v); ok {
		return t.UTC().Format(time.RFC3339)
	}
	return ""
}
