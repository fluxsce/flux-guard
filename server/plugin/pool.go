package plugin

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// 空闲连接最多留这么多。正在用的连接不占这个名额，多笔调用仍然各走各的。
	maxIdleConns = 32
	maxIdleAge   = 30 * time.Second
)

type idleConn struct {
	conn net.Conn
	at   time.Time
}

// connPool 复用空闲连接。取出后这条连接上同时只跑一笔调用。
type connPool struct {
	dial func(context.Context) (net.Conn, error)

	mu   sync.Mutex
	idle []idleConn
}

func newConnPool(dialFn func(context.Context) (net.Conn, error)) *connPool {
	return &connPool{dial: dialFn}
}

func (p *connPool) get(ctx context.Context) (conn net.Conn, reused bool, err error) {
	if p == nil {
		return nil, false, fmt.Errorf("插件未启动")
	}
	var stale []net.Conn
	p.mu.Lock()
	now := time.Now()
	for len(p.idle) > 0 {
		last := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		if now.Sub(last.at) > maxIdleAge {
			stale = append(stale, last.conn)
			continue
		}
		p.mu.Unlock()
		for _, item := range stale {
			_ = item.Close()
		}
		return last.conn, true, nil
	}
	p.mu.Unlock()
	for _, item := range stale {
		_ = item.Close()
	}
	c, err := p.dial(ctx)
	return c, false, err
}

func (p *connPool) put(conn net.Conn) {
	if conn == nil {
		return
	}
	if p == nil {
		_ = conn.Close()
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle) >= maxIdleConns {
		_ = conn.Close()
		return
	}
	p.idle = append(p.idle, idleConn{conn: conn, at: time.Now()})
}

func (p *connPool) closeAll() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, item := range p.idle {
		_ = item.conn.Close()
	}
	p.idle = nil
}
