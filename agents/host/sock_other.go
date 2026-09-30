//go:build !linux

package host

import (
	"context"

	"github.com/shirou/gopsutil/v4/net"
)

func readSockets(ctx context.Context) socketSnap {
	if err := ctx.Err(); err != nil {
		return socketSnap{}
	}
	conns, err := net.ConnectionsWithContext(ctx, "tcp")
	if err != nil {
		return socketSnap{}
	}
	var snap socketSnap
	snap.ok = true
	for _, item := range conns {
		addTCPState(&snap, item.Status)
	}
	if udp, err := net.ConnectionsWithContext(ctx, "udp"); err == nil {
		snap.udpInUse = uint64(len(udp))
		snap.udpOK = true
	}
	fillTCPInUse(&snap)
	return snap
}
