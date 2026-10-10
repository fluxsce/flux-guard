package plugin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"fluxguard/protocol"
)

const (
	statusStopped = "stopped"
	statusRunning = "running"
	statusExited  = "exited"
	stopWait      = 2 * time.Second
	maxStderr     = 4096
	pluginAddrEnv = "FG_PLUGIN_ADDRESS"
)

// service 是中央按 manifest 拉起并保持的一个外置插件进程。
// 进程监听本机管道。每笔调用自己拨一条连接，采集和操作可以同时进行。
type service struct {
	code string
	dir  string
	argv []string
	addr string

	startMu sync.Mutex
	stateMu sync.Mutex
	pool    *connPool
	cmd     *exec.Cmd
	errBuf  bytes.Buffer
	status  string
	hold    bool
	pid     int
	done    chan struct{}
}

func newService(code, dir string, argv []string, addr string) *service {
	s := &service{code: code, dir: dir, argv: argv, addr: addr, status: statusStopped}
	s.pool = newConnPool(func(ctx context.Context) (net.Conn, error) {
		return dialLocal(ctx, s.addr)
	})
	return s
}

func (s *service) snapshot() (string, int, bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.status, s.pid, s.hold
}

// stop 停掉进程。已经在途的连接会断开，其它调用不会再把它拉起来，直到 Start 清掉 hold。
func (s *service) stop() {
	s.stateMu.Lock()
	s.hold = true
	cmd := s.cmd
	done := s.done
	addr := s.addr
	s.stateMu.Unlock()
	s.pool.closeAll()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if done != nil {
		timer := time.NewTimer(stopWait)
		select {
		case <-done:
		case <-timer.C:
		}
		timer.Stop()
	}
	cleanupAddr(addr)
	s.stateMu.Lock()
	s.status = statusStopped
	s.cmd = nil
	s.pid = 0
	s.done = nil
	s.stateMu.Unlock()
}

func (s *service) start() error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.stateMu.Lock()
	if s.status == statusRunning {
		s.stateMu.Unlock()
		return nil
	}
	if s.hold {
		s.stateMu.Unlock()
		return fmt.Errorf("插件已停止")
	}
	// 进程自己退出后，下一次调用再拉起。只有 Stop 留下的 hold 不会自动再启动。
	if s.status == statusExited {
		s.status = statusStopped
		s.cmd = nil
		s.pid = 0
	}
	if len(s.argv) == 0 || s.addr == "" {
		s.stateMu.Unlock()
		return fmt.Errorf("插件启动失败")
	}
	cleanupAddr(s.addr)
	cmd := exec.Command(s.argv[0], s.argv[1:]...)
	cmd.Dir = s.dir
	cmd.Env = childEnv(s.addr)
	cmd.Stdout = io.Discard
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.stateMu.Unlock()
		return fmt.Errorf("插件启动失败")
	}
	if err := cmd.Start(); err != nil {
		s.stateMu.Unlock()
		return fmt.Errorf("插件启动失败")
	}
	done := make(chan struct{})
	s.cmd = cmd
	s.done = done
	s.pid = cmd.Process.Pid
	s.status = statusRunning
	s.errBuf.Reset()
	s.stateMu.Unlock()
	go func() {
		_, _ = io.Copy(&capped{max: maxStderr, buf: &s.errBuf}, stderr)
	}()
	go func() {
		_ = cmd.Wait()
		s.stateMu.Lock()
		if s.cmd == cmd && s.status == statusRunning {
			s.status = statusExited
			s.pid = 0
		}
		s.stateMu.Unlock()
		close(done)
	}()
	readyCtx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	if err := waitReady(readyCtx, s.addr); err != nil {
		_ = cmd.Process.Kill()
		timer := time.NewTimer(stopWait)
		select {
		case <-done:
		case <-timer.C:
		}
		timer.Stop()
		s.stateMu.Lock()
		if s.cmd == cmd {
			s.status = statusStopped
			s.cmd = nil
			s.pid = 0
			s.done = nil
		}
		s.stateMu.Unlock()
		return fmt.Errorf("插件启动失败")
	}
	return nil
}

// roundTrip 发出一帧并读回一帧。空闲连接会留下来给下一笔用。
// 正在进行的调用各占一条连接。这一笔超时只关掉自己的连接。
func (s *service) roundTrip(ctx context.Context, method string, params any, secret string, dest any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	id, err := newCallID()
	if err != nil {
		return err
	}
	// 同一次调用的跟踪号现在与调用号相同。调度以后可以把一轮采集收成同一个 traceId。
	req := protocol.ExtRequest{V: protocol.ExtVersion, ID: id, TraceID: id, Method: method, Params: raw}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := s.start(); err != nil {
		return err
	}
	conn, reused, err := s.pool.get(ctx)
	if err != nil {
		return s.dialErr(ctx, req.TraceID)
	}
	wrote, callErr := s.exchange(ctx, conn, payload, req, secret, dest)
	if callErr == nil {
		s.pool.put(conn)
		return nil
	}
	_ = conn.Close()
	// 池里的连接若在写出之前就断了，换一条新的再发一次。
	// 已经写出去再失败时不再发，避免维护操作做两遍。
	if !reused || wrote {
		return callErr
	}
	fresh, dialErr := dialLocal(ctx, s.addr)
	if dialErr != nil {
		return s.dialErr(ctx, req.TraceID)
	}
	_, callErr = s.exchange(ctx, fresh, payload, req, secret, dest)
	if callErr != nil {
		_ = fresh.Close()
		return callErr
	}
	s.pool.put(fresh)
	return nil
}

func (s *service) exchange(ctx context.Context, conn net.Conn, payload []byte, req protocol.ExtRequest, secret string, dest any) (bool, error) {
	if err := protocol.WriteExtFrame(conn, payload); err != nil {
		return false, &protocol.ExtFailure{Code: protocol.ExtCodeLost, Message: "插件已退出", TraceID: req.TraceID}
	}
	type got struct {
		frame []byte
		err   error
	}
	ch := make(chan got, 1)
	go func() {
		frame, readErr := protocol.ReadExtFrame(conn)
		ch <- got{frame, readErr}
	}()
	select {
	case <-ctx.Done():
		return true, &protocol.ExtFailure{Code: protocol.ExtCodeTimeout, Message: "插件超时", TraceID: req.TraceID}
	case result := <-ch:
		if errors.Is(result.err, protocol.ErrExtFrameTooLarge) {
			return true, &protocol.ExtFailure{Code: protocol.ExtCodeInvalidRequest, Message: "插件应答过大", TraceID: req.TraceID}
		}
		if result.err != nil && len(result.frame) == 0 {
			return true, &protocol.ExtFailure{Code: protocol.ExtCodeLost, Message: "插件已退出", TraceID: req.TraceID}
		}
		var reply protocol.ExtResponse
		if err := json.Unmarshal(result.frame, &reply); err != nil {
			return true, &protocol.ExtFailure{Code: protocol.ExtCodeInvalidRequest, Message: "插件应答无效", TraceID: req.TraceID}
		}
		if err := protocol.CheckExtResponse(req, reply); err != nil {
			return true, err
		}
		if !reply.OK {
			code := reply.ErrorCode
			if code == "" {
				code = protocol.ExtCodeInternal
			}
			message := redact(reply.Error, secret)
			if message == "" {
				message = "插件应答无效"
				code = protocol.ExtCodeInvalidRequest
			}
			return true, &protocol.ExtFailure{Code: code, Message: message, TraceID: reply.TraceID}
		}
		if err := protocol.DecodeExtResult(reply.Result, dest); err != nil {
			return true, &protocol.ExtFailure{Code: protocol.ExtCodeInvalidRequest, Message: "插件应答无效", TraceID: reply.TraceID}
		}
		return true, nil
	}
}

func (s *service) dialErr(ctx context.Context, traceID string) error {
	if ctx.Err() != nil {
		return &protocol.ExtFailure{Code: protocol.ExtCodeTimeout, Message: "插件超时", TraceID: traceID}
	}
	s.stateMu.Lock()
	status := s.status
	s.stateMu.Unlock()
	if status != statusRunning {
		return &protocol.ExtFailure{Code: protocol.ExtCodeLost, Message: "插件已退出", TraceID: traceID}
	}
	return &protocol.ExtFailure{Code: protocol.ExtCodeUnavailable, Message: "插件未启动", TraceID: traceID}
}

func waitReady(ctx context.Context, addr string) error {
	for {
		conn, err := dialLocal(ctx, addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func childEnv(addr string) []string {
	key := pluginAddrEnv + "="
	src := os.Environ()
	out := make([]string, 0, len(src)+1)
	for _, item := range src {
		if strings.HasPrefix(item, key) {
			continue
		}
		out = append(out, item)
	}
	return append(out, key+addr)
}

func newCallID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	if !protocol.ValidExtID(id) {
		return "", fmt.Errorf("插件应答无效")
	}
	return id, nil
}

type capped struct {
	buf  *bytes.Buffer
	max  int
	over bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.buf == nil {
		return len(p), nil
	}
	room := c.max - c.buf.Len()
	if room <= 0 {
		c.over = true
		return len(p), nil
	}
	if len(p) > room {
		_, _ = c.buf.Write(p[:room])
		c.over = true
		return len(p), nil
	}
	_, _ = c.buf.Write(p)
	return len(p), nil
}

func redact(text, secret string) string {
	if secret == "" || text == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "")
}
