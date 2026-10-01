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
	parallel  = 8
	probeWait = 8 * time.Second
)

// Loop 按间隔拨已启用的连接。取消 ctx 后停止，不再开新的拨号。
func Loop(ctx context.Context, db *dbx.DB) {
	if db == nil {
		return
	}
	run(ctx, db)
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run(ctx, db)
		}
	}
}

var inflight sync.Map

func run(ctx context.Context, db *dbx.DB) {
	due, err := conn.Due(ctx, db)
	if err != nil || len(due) == 0 {
		return
	}
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for _, item := range due {
		if ctx.Err() != nil {
			break
		}
		key := item.TenantID + "/" + item.ComponentID
		if _, loaded := inflight.LoadOrStore(key, true); loaded {
			continue
		}
		select {
		case <-ctx.Done():
			inflight.Delete(key)
			return
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(item conn.Open, key string) {
			defer wg.Done()
			defer inflight.Delete(key)
			defer func() { <-sem }()
			pctx, cancel := context.WithTimeout(ctx, probeWait)
			defer cancel()
			_, _ = runtime.Probe(pctx, db, item)
		}(item, key)
	}
	wg.Wait()
}
