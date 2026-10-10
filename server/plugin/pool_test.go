package plugin

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestPoolReusesIdleConn(t *testing.T) {
	var dials int
	p := newConnPool(func(context.Context) (net.Conn, error) {
		dials++
		left, right := net.Pipe()
		t.Cleanup(func() { _ = right.Close() })
		return left, nil
	})
	t.Cleanup(p.closeAll)
	ctx := context.Background()
	first, reused, err := p.get(ctx)
	if err != nil || reused || dials != 1 {
		t.Fatalf("first %v reused=%v dials=%d", err, reused, dials)
	}
	p.put(first)
	second, reused, err := p.get(ctx)
	if err != nil || !reused || dials != 1 || second != first {
		t.Fatalf("reuse %v reused=%v dials=%d", err, reused, dials)
	}
	p.put(second)
	p.mu.Lock()
	p.idle[0].at = time.Now().Add(-time.Minute)
	p.mu.Unlock()
	third, reused, err := p.get(ctx)
	if err != nil || reused || dials != 2 {
		t.Fatalf("stale %v reused=%v dials=%d", err, reused, dials)
	}
	_ = third.Close()
}
