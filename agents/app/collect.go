// Package app 是应用代理的 Go 参考实现：采集当前进程的运行时，再自行上报。
// JVM 代理在 agents/java。其它语言的应用代理自己采集、按 protocol 上报。
package app

import (
	"context"
	"runtime"
	"time"

	"fluxguard/protocol"
)

// Code 是协议里的代理种类。
const Code = protocol.AgentApp

// Collect 读取本进程的 Go 运行时。JVM 由 agents/java 采集。
func Collect(ctx context.Context) (protocol.CollectResult, error) {
	if err := ctx.Err(); err != nil {
		return protocol.CollectResult{}, err
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	now := time.Now().UTC()
	labels := map[string]string{"source": "go"}
	return protocol.CollectResult{Samples: []protocol.Sample{
		{MetricName: "app.runtime.goroutine.count", MetricValue: float64(runtime.NumGoroutine()), Labels: labels, ObservedAt: now},
		{MetricName: "app.runtime.heap.alloc.bytes", MetricValue: float64(mem.HeapAlloc), Labels: labels, ObservedAt: now},
	}}, nil
}
