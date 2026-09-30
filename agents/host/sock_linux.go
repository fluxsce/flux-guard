//go:build linux

package host

import (
	"context"
	"os"
)

func readSockets(ctx context.Context) socketSnap {
	if err := ctx.Err(); err != nil {
		return socketSnap{}
	}
	var snap socketSnap
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		countTCPTable(string(body), &snap)
	}
	for _, path := range []string{"/proc/net/sockstat", "/proc/net/sockstat6"} {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parseSockstat(string(body), &snap)
	}
	if body, err := os.ReadFile("/proc/net/snmp"); err == nil {
		if n, ok := parseTCPRetrans(string(body)); ok {
			snap.retrans = n
			snap.hasRetrans = true
		}
	}
	fillTCPInUse(&snap)
	return snap
}
