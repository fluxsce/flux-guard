package validate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/server/store"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

// CleanID 校验或生成编码。
func CleanID(value string, creating bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if !creating {
			return "", fmt.Errorf("缺少编码")
		}
		return id.New(), nil
	}
	if !idPattern.MatchString(value) {
		return "", fmt.Errorf("编码只允许字母、数字、下划线和短横线，最长 32 位")
	}
	return value, nil
}

// CleanName 校验名称。
func CleanName(value, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("请填写%s", label)
	}
	if utf8.RuneCountInString(value) > 100 {
		return "", fmt.Errorf("%s过长", label)
	}
	return value, nil
}

// CleanNote 校验备注。
func CleanNote(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > 500 {
		return "", fmt.Errorf("备注过长")
	}
	return value, nil
}

// Flag 规范化 Y/N 标志。
func Flag(value, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value != "Y" && value != "N" {
		return fallback
	}
	return value
}

// AsText 将库字段转为字符串。
func AsText(value any) string {
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

// AsFloat 将库字段转为浮点数。
func AsFloat(value any) (float64, bool) {
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

// AsInt 将库字段转为整数。
func AsInt(value any) int {
	number, ok := AsFloat(value)
	if !ok {
		return 0
	}
	return int(number)
}

// Clip 按 rune 截断字符串。
func Clip(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

// SplitList 拆分逗号/分号/空白分隔的列表。
func SplitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '、' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	var out []string
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

// StoreTime 返回当前 UTC 时间。
func StoreTime() time.Time {
	return time.Now().UTC()
}

// SilenceWindow 校验静默窗口。两端都空表示不静默。
func SilenceWindow(startText, endText string) (string, string, error) {
	startText = strings.TrimSpace(startText)
	endText = strings.TrimSpace(endText)
	if startText == "" && endText == "" {
		return "", "", nil
	}
	if startText == "" || endText == "" {
		return "", "", ErrText("静默需要同时填写开始和结束")
	}
	start, err := parseWhen(startText)
	if err != nil {
		return "", "", ErrText("静默开始时间无效")
	}
	end, err := parseWhen(endText)
	if err != nil {
		return "", "", ErrText("静默结束时间无效")
	}
	if !end.After(start) {
		return "", "", ErrText("静默结束要晚于开始")
	}
	return store.FormatTime(start), store.FormatTime(end), nil
}

func parseWhen(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("请填写时间")
	}
	if when, ok := store.ParseTime(value); ok {
		return when, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04"} {
		when, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return when.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("时间格式无效")
}
