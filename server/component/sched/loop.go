package sched

import (
	"context"
	"sync"
	"time"

	"fluxguard/server/component/conn"
	"fluxguard/server/component/runtime"
	dbx "github.com/fluxsce/dbx/db"
)

const (
	tickEvery = 5 * time.Second
	// parallel 是同时在途的采集上限。名额满了的留到下一轮，慢的一批不挡住后面的调度。
	parallel = 32
	// 访问慢时给够时间做完。短于 30 秒的采集间隔也至少等 30 秒，避免刚连上就被掐掉。
	// 超过 2 分钟仍无应答视为这次失败，名额留给其它实例。
	probeWaitMin = 30 * time.Second
	probeWaitMax = 2 * time.Minute
)

// Loop 按间隔拨已启用的连接。取消 ctx 后不再开新的拨号，并等已经发出的采集结束。
func Loop(ctx context.Context, db *dbx.DB) {
	if db == nil {
		return
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	run(ctx, db, &wg)
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run(ctx, db, &wg)
		}
	}
}

var (
	inflight sync.Map
	slots    = make(chan struct{}, parallel)
)

func run(ctx context.Context, db *dbx.DB, wg *sync.WaitGroup) {
	due, err := conn.Due(ctx, db)
	if err != nil || len(due) == 0 {
		return
	}
	for _, item := range due {
		if ctx.Err() != nil {
			return
		}
		key := item.TenantID + "/" + item.ComponentID
		select {
		case slots <- struct{}{}:
		default:
			// 这一轮名额已满。实例仍是到期的，下一轮再拨。
			continue
		}
		if _, loaded := inflight.LoadOrStore(key, true); loaded {
			<-slots
			continue
		}
		wg.Add(1)
		go func(item conn.Open, key string) {
			defer wg.Done()
			defer inflight.Delete(key)
			defer func() { <-slots }()
			pctx, cancel := context.WithTimeout(ctx, probeTimeout(item.IntervalSec))
			defer cancel()
			_, _ = runtime.Probe(pctx, db, item)
		}(item, key)
	}
}

// probeTimeout 按这条连接的采集间隔给出本次上限。间隔很短时仍保留最短等待，间隔很长时不超过上限。
func probeTimeout(intervalSec int) time.Duration {
	wait := time.Duration(intervalSec) * time.Second
	if wait < probeWaitMin {
		return probeWaitMin
	}
	if wait > probeWaitMax {
		return probeWaitMax
	}
	return wait
}
