package policy

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cron 五段 cron：分 时 日 月 周。按 UTC 匹配。日和周都写了具体值时，命中任意一段即可。
type Cron struct {
	min    [60]bool
	hour   [24]bool
	dom    [32]bool
	month  [13]bool
	dow    [7]bool
	domAny bool
	dowAny bool
}

// ParseCron 解析任意标准五段表达式。支持 *、数字、区间、列表和步长，月份与星期可用英文缩写。
func ParseCron(expr string) (Cron, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return Cron{}, fmt.Errorf("cron 须为五段")
	}
	var c Cron
	var err error
	if c.min, err = parseField(fields[0], 0, 59); err != nil {
		return Cron{}, fmt.Errorf("分钟: %w", err)
	}
	if c.hour, err = parseHour(fields[1]); err != nil {
		return Cron{}, err
	}
	c.domAny = fields[2] == "*"
	if c.dom, err = parseDOM(fields[2]); err != nil {
		return Cron{}, err
	}
	if c.month, err = parseMonth(fields[3]); err != nil {
		return Cron{}, err
	}
	c.dowAny = fields[4] == "*"
	if c.dow, err = parseDow(fields[4]); err != nil {
		return Cron{}, err
	}
	return c, nil
}

// Match 当前时刻是否落在表达式上。比较到分钟。
func (c Cron) Match(t time.Time) bool {
	t = t.UTC()
	if !c.min[t.Minute()] || !c.hour[t.Hour()] || !c.month[int(t.Month())] {
		return false
	}
	domOK := c.dom[t.Day()]
	dowOK := c.dow[int(t.Weekday())]
	if c.domAny && c.dowAny {
		return true
	}
	if c.domAny {
		return dowOK
	}
	if c.dowAny {
		return domOK
	}
	return domOK || dowOK
}

func parseHour(field string) ([24]bool, error) {
	wide, err := parseField(field, 0, 23)
	if err != nil {
		return [24]bool{}, fmt.Errorf("小时: %w", err)
	}
	var out [24]bool
	copy(out[:], wide[:24])
	return out, nil
}

func parseDOM(field string) ([32]bool, error) {
	wide, err := parseField(field, 1, 31)
	if err != nil {
		return [32]bool{}, fmt.Errorf("日: %w", err)
	}
	var out [32]bool
	copy(out[:], wide[:32])
	return out, nil
}

func parseMonth(field string) ([13]bool, error) {
	field = replaceNames(field, monthNames, false)
	wide, err := parseField(field, 1, 12)
	if err != nil {
		return [13]bool{}, fmt.Errorf("月: %w", err)
	}
	var out [13]bool
	copy(out[:], wide[:13])
	return out, nil
}

func parseDow(field string) ([7]bool, error) {
	field = replaceNames(field, dowNames, true)
	wide, err := parseField(field, 0, 7)
	if err != nil {
		return [7]bool{}, fmt.Errorf("周: %w", err)
	}
	var out [7]bool
	for i := 0; i <= 6; i++ {
		out[i] = wide[i]
	}
	if wide[7] {
		out[0] = true
	}
	return out, nil
}

func parseField(field string, min, max int) ([60]bool, error) {
	var bits [60]bool
	if max >= len(bits) {
		return bits, fmt.Errorf("范围过大")
	}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return bits, fmt.Errorf("空段")
		}
		if err := fillPart(&bits, part, min, max); err != nil {
			return bits, err
		}
	}
	return bits, nil
}

func fillPart(bits *[60]bool, part string, min, max int) error {
	step := 1
	base := part
	if i := strings.Index(part, "/"); i >= 0 {
		base = part[:i]
		n, err := strconv.Atoi(part[i+1:])
		if err != nil || n <= 0 {
			return fmt.Errorf("步长无效")
		}
		step = n
	}
	start, end := min, max
	if base != "*" {
		if i := strings.Index(base, "-"); i >= 0 {
			a, err1 := strconv.Atoi(base[:i])
			b, err2 := strconv.Atoi(base[i+1:])
			if err1 != nil || err2 != nil || a > b {
				return fmt.Errorf("区间无效")
			}
			start, end = a, b
		} else {
			n, err := strconv.Atoi(base)
			if err != nil {
				return fmt.Errorf("数字无效")
			}
			start, end = n, n
		}
	}
	if start < min || end > max {
		return fmt.Errorf("超出 %d-%d", min, max)
	}
	for n := start; n <= end; n += step {
		bits[n] = true
	}
	return nil
}

var monthNames = []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
var dowNames = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

func replaceNames(field string, names []string, zeroBased bool) string {
	lower := strings.ToLower(field)
	for i, name := range names {
		n := i + 1
		if zeroBased {
			n = i
		}
		lower = strings.ReplaceAll(lower, name, strconv.Itoa(n))
	}
	return lower
}
