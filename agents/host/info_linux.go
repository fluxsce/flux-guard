//go:build linux

package host

import (
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func osVersion() string {
	body, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	var name, version, pretty string
	for _, line := range strings.Split(string(body), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "PRETTY_NAME":
			pretty = value
		case "NAME":
			name = value
		case "VERSION":
			version = value
		}
	}
	if pretty != "" {
		return pretty
	}
	return strings.TrimSpace(name + " " + version)
}

func bootTime() time.Time {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return time.Time{}
	}
	return bootFromUptime(time.Duration(info.Uptime) * time.Second)
}
