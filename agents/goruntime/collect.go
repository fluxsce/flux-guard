// Package goruntime 上报当前这个 Go 进程自己的运行时。
// 它不扫描机器上的应用，也不换包。应用清单和更新在 agents/appside。
package goruntime

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"time"

	"fluxguard/protocol"
)

// Code 是协议里的代理种类。
const Code = protocol.AgentGo

var started = time.Now().UTC()

// Collect 读取本进程的身份、协程、堆和 GC。进程退出后这一轮不再发出。
func Collect(ctx context.Context) (protocol.CollectResult, error) {
	if err := ctx.Err(); err != nil {
		return protocol.CollectResult{}, err
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	host, _ := os.Hostname()
	now := time.Now().UTC()
	snap := &protocol.Go{
		GoVersion:      runtime.Version(),
		ProcessID:      strconv.Itoa(os.Getpid()),
		StartTime:      started,
		HostName:       host,
		NumCPU:         runtime.NumCPU(),
		GoroutineCount: runtime.NumGoroutine(),
		HeapAllocBytes: mem.HeapAlloc,
		HeapSysBytes:   mem.HeapSys,
		HeapObjects:    mem.HeapObjects,
		GCCount:        mem.NumGC,
		GCPauseMs:      float64(mem.PauseTotalNs) / 1e6,
	}
	labels := map[string]string{"source": "go"}
	return protocol.CollectResult{
		Go: snap,
		Samples: []protocol.Sample{
			{MetricName: "go.runtime.goroutine.count", MetricValue: float64(snap.GoroutineCount), Labels: labels, ObservedAt: now},
			{MetricName: "go.runtime.heap.alloc.bytes", MetricValue: float64(snap.HeapAllocBytes), Labels: labels, ObservedAt: now},
			{MetricName: "go.runtime.heap.sys.bytes", MetricValue: float64(snap.HeapSysBytes), Labels: labels, ObservedAt: now},
			{MetricName: "go.runtime.heap.objects", MetricValue: float64(snap.HeapObjects), Labels: labels, ObservedAt: now},
			{MetricName: "go.runtime.gc.count", MetricValue: float64(snap.GCCount), Labels: labels, ObservedAt: now},
			{MetricName: "go.runtime.gc.pause.ms", MetricValue: snap.GCPauseMs, Labels: labels, ObservedAt: now},
		},
	}, nil
}
