package policy

import (
	"time"

	"fluxguard/server/job/contract"
	"fluxguard/server/limits"
)

// nextSearchLimit 是 cron 向后查找的上限。再远视为没有下一次。
const nextSearchLimit = 366 * 24 * time.Hour

// NextStart 计算下一次执行开始时间。last 是上一次开始时刻，零值表示还没执行。
// 手动没有下一次。指定时间已经为这个时刻执行过，也没有下一次。
// 间隔从上次开始加上间隔分钟；还没执行或已经到期时，落在 now 所在的分钟。
// cron 从当前分钟起向后找第一个尚未消费的命中分钟，时间按 UTC。
func NextStart(sched Schedule, last, now time.Time) (time.Time, bool) {
	now = now.UTC()
	if !last.IsZero() {
		last = last.UTC()
	}
	minute := now.Truncate(time.Minute)
	switch normalizeMode(sched.Mode) {
	case string(contract.TriggerManual):
		return time.Time{}, false
	case string(contract.TriggerAt):
		at, err := ParseAt(sched.RunAt)
		if err != nil {
			return time.Time{}, false
		}
		if !last.IsZero() && !last.Before(at) {
			return time.Time{}, false
		}
		if now.Before(at) {
			return at, true
		}
		return minute, true
	case string(contract.TriggerCron):
		expr, err := ParseCron(sched.Cron)
		if err != nil {
			return time.Time{}, false
		}
		start := minute
		if !last.IsZero() && !last.Before(start) {
			start = start.Add(time.Minute)
		}
		limit := start.Add(nextSearchLimit)
		for t := start; t.Before(limit); t = t.Add(time.Minute) {
			if expr.Match(t) {
				return t, true
			}
		}
		return time.Time{}, false
	default:
		interval := sched.IntervalMinutes
		if interval <= 0 {
			interval = limits.RetentionIntervalDefaultMin
		}
		if last.IsZero() {
			return minute, true
		}
		next := last.Add(time.Duration(interval) * time.Minute)
		if now.Before(next) {
			return next, true
		}
		return minute, true
	}
}
