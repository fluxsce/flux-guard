package host

import (
	"strconv"
	"strings"
)

// parseFileNr 解析 /proc/sys/fs/file-nr。
// 常见三列：已分配、空闲（2.6 起恒为 0）、上限。
// 两列时按已分配、上限读，避免把恒为 0 的空闲列当成已用数。
func parseFileNr(body string) (allocated, maximum uint64, ok bool) {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return 0, 0, false
	}
	allocated, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	maxField := ""
	switch len(fields) {
	case 1:
		return allocated, 0, true
	case 2:
		maxField = fields[1]
	default:
		maxField = fields[2]
	}
	maximum, err = strconv.ParseUint(maxField, 10, 64)
	if err != nil {
		return allocated, 0, true
	}
	return allocated, maximum, true
}
