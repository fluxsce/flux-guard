//go:build windows

package host

import (
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

var procGetTickCount64 = syscall.NewLazyDLL("kernel32.dll").NewProc("GetTickCount64")

func osVersion() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	product, _, _ := key.GetStringValue("ProductName")
	display, _, _ := key.GetStringValue("DisplayVersion")
	build, _, _ := key.GetStringValue("CurrentBuild")
	text := product
	if display != "" {
		text += " " + display
	}
	if build != "" {
		text += " (" + build + ")"
	}
	return text
}

func bootTime() time.Time {
	ticks, _, _ := procGetTickCount64.Call()
	if ticks == 0 {
		return time.Time{}
	}
	return bootFromUptime(time.Duration(ticks) * time.Millisecond)
}
