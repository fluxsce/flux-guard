package shell

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"fluxguard/server/utils/const/limits"
)

const (
	maxIn  = limits.ShellMaxIn
	maxOut = limits.ShellMaxOut
)

// lane 是一路终端在本进程里的输入输出缓冲。数据库只记会话，字节不落库。
type lane struct {
	mu      sync.Mutex
	cond    *sync.Cond
	in      []byte
	out     []byte
	base    int
	closed  bool
	cols    int
	rows    int
	sizeGen int
	typed   []rune
	esc     bool
}

func newLane() *lane {
	l := &lane{}
	l.cond = sync.NewCond(&l.mu)
	return l
}

type hub struct {
	mu    sync.Mutex
	lanes map[string]*lane
	seen  map[string]time.Time
}

func newHub() *hub {
	return &hub{lanes: map[string]*lane{}, seen: map[string]time.Time{}}
}

var pipes = newHub()

func (h *hub) lane(id string) *lane {
	h.mu.Lock()
	defer h.mu.Unlock()
	l := h.lanes[id]
	if l == nil {
		l = newLane()
		h.lanes[id] = l
	}
	return l
}

func (h *hub) get(id string) *lane {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lanes[id]
}

func (h *hub) touch(id string) {
	h.mu.Lock()
	h.seen[id] = time.Now()
	h.mu.Unlock()
}

func (h *hub) idle(id string, limit time.Duration) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	at, ok := h.seen[id]
	if !ok {
		h.seen[id] = time.Now()
		return false
	}
	return time.Since(at) > limit
}

func (h *hub) backdate(id string, at time.Time) {
	h.mu.Lock()
	h.seen[id] = at
	h.mu.Unlock()
}

func (h *hub) shutdown(id string) {
	h.mu.Lock()
	l := h.lanes[id]
	delete(h.lanes, id)
	delete(h.seen, id)
	h.mu.Unlock()
	if l == nil {
		return
	}
	l.mu.Lock()
	l.closed = true
	l.in = nil
	l.out = nil
	l.typed = nil
	l.cond.Broadcast()
	l.mu.Unlock()
}

func (l *lane) pushIn(p []byte) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return false
	}
	if len(l.in)+len(p) > maxIn {
		return false
	}
	l.in = append(l.in, p...)
	l.cond.Broadcast()
	return true
}

func (l *lane) takeIn(ctx context.Context, wait time.Duration) (data []byte, closed bool, cols, rows int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	deadline := time.Now().Add(wait)
	stop := context.AfterFunc(ctx, func() { l.cond.Broadcast() })
	defer stop()
	gen := l.sizeGen
	for len(l.in) == 0 && !l.closed && ctx.Err() == nil && time.Now().Before(deadline) && l.sizeGen == gen {
		remain := time.Until(deadline)
		timer := time.AfterFunc(remain, func() { l.cond.Broadcast() })
		l.cond.Wait()
		timer.Stop()
	}
	data = append([]byte(nil), l.in...)
	l.in = nil
	cols, rows = l.cols, l.rows
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	return data, l.closed, cols, rows
}

func (l *lane) setSize(cols, rows int) bool {
	if cols < 1 || rows < 1 || cols > 500 || rows > 200 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cols == cols && l.rows == rows {
		return true
	}
	l.cols = cols
	l.rows = rows
	l.sizeGen++
	l.cond.Broadcast()
	return true
}

// auditLine 把按键拼成一行。回车时返回这一行，供审计；方向键和单独按键不记。
func (l *lane) auditLine(text string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var done string
	for _, r := range text {
		if l.esc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '~' {
				l.esc = false
			}
			continue
		}
		switch r {
		case 0x1b:
			l.esc = true
		case '\r', '\n':
			done = strings.TrimSpace(string(l.typed))
			l.typed = l.typed[:0]
		case '\b', 0x7f:
			if len(l.typed) > 0 {
				l.typed = l.typed[:len(l.typed)-1]
			}
		case 0x03:
			l.typed = l.typed[:0]
		default:
			if r >= 0x20 && len(l.typed) < 500 {
				l.typed = append(l.typed, r)
			}
		}
	}
	return done
}

func (l *lane) pushOut(p []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = append(l.out, p...)
	if len(l.out) > maxOut {
		drop := len(l.out) - maxOut
		l.base += drop
		l.out = append([]byte(nil), l.out[drop:]...)
	}
	l.cond.Broadcast()
}

func (l *lane) readOut(offset int) (text string, next int, closed, skipped bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	skipped = offset < l.base
	if offset < l.base {
		offset = l.base
	}
	start := offset - l.base
	if start > len(l.out) {
		start = len(l.out)
	}
	chunk := l.out[start:]
	return string(chunk), l.base + len(l.out), l.closed, skipped
}

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}
