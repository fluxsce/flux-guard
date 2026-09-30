//go:build linux

package host

import (
	"os"
	"strconv"
	"strings"
)

func fileDescriptorLimits() (allocated, maximum uint64, ok bool) {
	body, err := os.ReadFile("/proc/sys/fs/file-nr")
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(string(body))
	if len(fields) < 3 {
		return 0, 0, false
	}
	allocated, err = strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	maximum, err = strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return allocated, maximum, true
}
