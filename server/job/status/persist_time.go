package status

import (
	"time"

	"fluxguard/server/job/policy"
)

func yn(ok bool) string {
	if ok {
		return "Y"
	}
	return "N"
}

func nextStartArg(s Status) any {
	if !s.Enabled {
		return nil
	}
	sched := policy.DecodeSchedule(policy.Get(s.TenantID, s.Code))
	if !sched.Enabled {
		return nil
	}
	var last time.Time
	if s.RanAtLeast {
		last = s.LastStart
		if last.IsZero() {
			last = s.LastEnd
		}
	}
	at, ok := policy.NextStart(sched, last, storeTimeNow())
	if !ok {
		return nil
	}
	return timeArg(at)
}

func timeArg(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	text := t.UTC().Format("2006-01-02 15:04:05")
	if text == "" {
		return nil
	}
	return text
}

func emptyTrigger(v string) string {
	if v == "" {
		return "manual"
	}
	return v
}

func textOf(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func storeTimeNow() time.Time { return time.Now().UTC() }
