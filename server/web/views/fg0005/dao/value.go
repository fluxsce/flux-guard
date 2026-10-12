package dao

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fluxguard/server/store"
)

func textOf(value any) string {
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
		return fmt.Sprint(typed)
	}
}

func numberOf(value any) (float64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case []byte:
		number, err := strconv.ParseFloat(string(typed), 64)
		return number, err == nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, false
		}
		number, err := strconv.ParseFloat(typed, 64)
		return number, err == nil
	default:
		number, err := strconv.ParseFloat(fmt.Sprint(typed), 64)
		return number, err == nil
	}
}

func intOf(value any) int {
	number, ok := numberOf(value)
	if !ok {
		return 0
	}
	return int(number)
}
