//go:build !windows

package plugin

import (
	"context"
	"net"
	"os"
	"time"
)

func listenLocal(addr string) (net.Listener, error) {
	_ = os.Remove(addr)
	return net.Listen("unix", addr)
}

func dialLocal(ctx context.Context, addr string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", addr)
}

func cleanupAddr(addr string) {
	_ = os.Remove(addr)
}

// dialTimeout 是启动探活的上限。自带运行时冷启动可能要几秒，管道一就绪就返回。
// 调用本身的超时由调用方的 context 决定。
const dialTimeout = 15 * time.Second
