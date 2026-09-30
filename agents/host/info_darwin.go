//go:build darwin

package host

import (
	"encoding/binary"
	"time"

	"golang.org/x/sys/unix"
)

func osVersion() string {
	product, err := unix.Sysctl("kern.osproductversion")
	if err != nil || product == "" {
		release, err := unix.Sysctl("kern.osrelease")
		if err != nil {
			return ""
		}
		return release
	}
	return "macOS " + product
}

func bootTime() time.Time {
	raw, err := unix.SysctlRaw("kern.boottime")
	if err != nil || len(raw) < 8 {
		return time.Time{}
	}
	sec := int64(binary.LittleEndian.Uint64(raw[:8]))
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}
