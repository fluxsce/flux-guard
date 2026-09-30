package host

import (
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	hostinfo "github.com/shirou/gopsutil/v4/host"

	"fluxguard/protocol"
)

// MachineID 返回操作系统机器编号。命令拉取用它向服务器标明这台机器。
func MachineID() string {
	return identity(0).MachineID
}

func identity(memTotal uint64) protocol.Host {
	name, _ := os.Hostname()
	item := protocol.Host{
		HostName:      name,
		OSName:        runtime.GOOS,
		OSVersion:     osVersion(),
		ArchName:      runtime.GOARCH,
		CPUCount:      runtime.NumCPU(),
		MemTotalBytes: memTotal,
		IPAddress:     firstIPv4(),
		IPList:        ipv4List(),
		BootTime:      bootTime(),
	}
	if info, err := hostinfo.Info(); err == nil {
		item.MachineID = clipField(info.HostID, 64)
		item.KernelVersion = clipField(info.KernelVersion, 100)
	}
	return item
}

func firstIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			if v4 := ipNet.IP.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return ""
}

func ipv4List() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			v4 := ipNet.IP.To4()
			if v4 == nil {
				continue
			}
			out = append(out, v4.String())
			if len(out) >= 8 {
				return clipField(strings.Join(out, ","), 255)
			}
		}
	}
	return clipField(strings.Join(out, ","), 255)
}

func clipField(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func bootFromUptime(uptime time.Duration) time.Time {
	if uptime <= 0 {
		return time.Time{}
	}
	return time.Now().UTC().Add(-uptime)
}
