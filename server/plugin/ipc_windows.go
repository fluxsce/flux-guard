//go:build windows

package plugin

import (
	"context"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

func listenLocal(addr string) (net.Listener, error) {
	return winio.ListenPipe(addr, &winio.PipeConfig{
		InputBufferSize:  64 * 1024,
		OutputBufferSize: 64 * 1024,
	})
}

func dialLocal(ctx context.Context, addr string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, addr)
}

func cleanupAddr(string) {}

// dialTimeout 是启动探活的上限。自带运行时冷启动可能要几秒，管道一就绪就返回。
// 调用本身的超时由调用方的 context 决定。
const dialTimeout = 15 * time.Second
