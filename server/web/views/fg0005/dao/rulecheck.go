package dao

import (
	"strconv"
	"strings"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0005/models"
)

func metricName(code string) string {
	switch code {
	case "cpu.usage":
		return "主机 CPU 使用率"
	case "mem.usage":
		return "内存使用率"
	case "disk.usage":
		return "磁盘使用率"
	case "disk.inode":
		return "inode 使用率"
	case "load.1":
		return "1 分钟负载"
	case "fd.usage":
		return "文件句柄使用率"
	case "agent.offline":
		return "代理离线分钟"
	default:
		return code
	}
}

func validMetric(code string) bool {
	switch code {
	case "cpu.usage", "mem.usage", "disk.usage", "disk.inode", "load.1", "fd.usage", "agent.offline":
		return true
	default:
		return false
	}
}

func validOp(op string) bool {
	switch op {
	case "gt", "gte", "lt", "lte":
		return true
	default:
		return false
	}
}

func opLabel(op string) string {
	switch op {
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	default:
		return op
	}
}

func breached(op string, value, threshold float64) bool {
	switch op {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	default:
		return false
	}
}

func formatNum(value float64) string {
	text := strconv.FormatFloat(value, 'f', 3, 64)
	text = strings.TrimRight(text, "0")
	return strings.TrimRight(text, ".")
}

func ruleSilenced(rule models.HostRule, now time.Time) bool {
	if rule.SilenceStart == "" || rule.SilenceEnd == "" {
		return false
	}
	start, ok := store.ParseTime(rule.SilenceStart)
	if !ok {
		return false
	}
	end, ok := store.ParseTime(rule.SilenceEnd)
	if !ok {
		return false
	}
	return !now.Before(start) && now.Before(end)
}
