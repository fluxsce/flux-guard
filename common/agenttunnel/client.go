// Package agenttunnel 是代理出站连中央会话隧道的客户端。
// 一条连接多路处理；通道处理器须自行限制缓冲并在 ctx 取消时退出，避免泄漏。
package agenttunnel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"fluxguard/common/logger"
	"fluxguard/protocol"
	"github.com/coder/websocket"
)

// Handler 处理中央下发的一帧（通常 open/close/data/control）。
type Handler func(ctx context.Context, send func(protocol.TunnelFrame) error, frame protocol.TunnelFrame)

// Config 隧道客户端配置。令牌与上报相同；禁止把令牌写入日志。
type Config struct {
	ServerURL  string
	AgentToken string
	AgentCode  string
	MachineID  string
}

// Client 维持出站隧道并分发通道。
type Client struct {
	cfg      Config
	mu       sync.Mutex
	handlers map[string]Handler
}

// New 创建客户端。
func New(cfg Config) *Client {
	return &Client{
		cfg:      cfg,
		handlers: map[string]Handler{},
	}
}

// Handle 登记通道；在 Run 前调用。
func (c *Client) Handle(channel string, fn Handler) {
	c.mu.Lock()
	c.handlers[channel] = fn
	c.mu.Unlock()
}

// Run 一直重连直到 ctx 取消。断线后指数退避，避免打爆中央。
func (c *Client) Run(ctx context.Context) {
	cfg := c.cfg
	cfg.ServerURL = strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	cfg.AgentToken = strings.TrimSpace(cfg.AgentToken)
	if cfg.ServerURL == "" || cfg.AgentToken == "" || cfg.MachineID == "" {
		logger.Warn(ctx, "tunnel: 缺少中央地址、令牌或机器编号，不开启会话隧道")
		return
	}
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if err := c.session(ctx, cfg); err != nil && ctx.Err() == nil {
			logger.Error(ctx, "tunnel: 会话结束，%s 后重连", backoff)
		} else {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (c *Client) session(ctx context.Context, cfg Config) error {
	u, err := url.Parse(cfg.ServerURL + protocol.TunnelPath)
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	q := u.Query()
	q.Set("agentCode", cfg.AgentCode)
	q.Set("machineId", cfg.MachineID)
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Set("Authorization", protocol.AuthorizationValue(cfg.AgentToken))
	ws, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	ws.SetReadLimit(512 << 10)

	sessCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sendCh := make(chan protocol.TunnelFrame, 64)
	var writeMu sync.Mutex
	send := func(frame protocol.TunnelFrame) error {
		select {
		case <-sessCtx.Done():
			return sessCtx.Err()
		case sendCh <- frame:
			return nil
		default:
			return errQueueFull
		}
	}

	go func() {
		defer cancel()
		for {
			select {
			case <-sessCtx.Done():
				return
			case frame := <-sendCh:
				raw, err := json.Marshal(frame)
				if err != nil {
					continue
				}
				writeMu.Lock()
				wctx, wcancel := context.WithTimeout(sessCtx, 10*time.Second)
				err = ws.Write(wctx, websocket.MessageText, raw)
				wcancel()
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	_ = send(protocol.TunnelFrame{Type: protocol.TunnelTypeHello})

	for {
		_, data, err := ws.Read(sessCtx)
		if err != nil {
			return err
		}
		var frame protocol.TunnelFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			continue
		}
		switch frame.Type {
		case protocol.TunnelTypePing:
			_ = send(protocol.TunnelFrame{Type: protocol.TunnelTypePong})
			continue
		case protocol.TunnelTypePong, protocol.TunnelTypeHello:
			continue
		}
		c.mu.Lock()
		fn := c.handlers[frame.Channel]
		c.mu.Unlock()
		if fn == nil {
			continue
		}
		if frame.Type == protocol.TunnelTypeOpen {
			go fn(sessCtx, send, frame)
			continue
		}
		fn(sessCtx, send, frame)
	}
}

var errQueueFull = errors.New("发送队列已满")
