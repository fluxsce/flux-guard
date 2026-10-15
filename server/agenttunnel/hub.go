// Package agenttunnel 是代理出站 WebSocket 会话隧道。
// 一条连接多路复用 shell / apptail；连接断开时取消该代理全部通道并释放缓冲，避免泄漏。
package agenttunnel

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"fluxguard/common/logger"
	"fluxguard/protocol"
	"fluxguard/server/utils/const/limits"
	"github.com/coder/websocket"
)

// Handler 处理某一通道上的入站帧（代理→中央）。
type Handler func(ctx context.Context, tenantID, agentID string, frame protocol.TunnelFrame)

// AttachHook 在代理隧道接入后回调（用于重推已打开的会话）。
type AttachHook func(tenantID, agentID string)

// Hub 按租户+代理登记在线隧道。
type Hub struct {
	mu       sync.Mutex
	conns    map[string]*Conn // key = tenantID + "\x00" + agentID
	handlers map[string]Handler
	onAttach []AttachHook
}

// Global 进程内单例，由 routes 注册。
var Global = NewHub()

// NewHub 创建空枢纽。
func NewHub() *Hub {
	return &Hub{
		conns:    map[string]*Conn{},
		handlers: map[string]Handler{},
	}
}

func connKey(tenantID, agentID string) string {
	return tenantID + "\x00" + agentID
}

// Handle 登记通道处理器；重复登记覆盖。
func (h *Hub) Handle(channel string, fn Handler) {
	h.mu.Lock()
	h.handlers[channel] = fn
	h.mu.Unlock()
}

// OnAttach 登记接入钩子（可多次）。
func (h *Hub) OnAttach(fn AttachHook) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	h.onAttach = append(h.onAttach, fn)
	h.mu.Unlock()
}

// Attach 登记一条已鉴权连接；同代理旧连接被替换并关闭。
func (h *Hub) Attach(tenantID, agentID string, c *Conn) {
	key := connKey(tenantID, agentID)
	h.mu.Lock()
	old := h.conns[key]
	h.conns[key] = c
	hooks := append([]AttachHook(nil), h.onAttach...)
	h.mu.Unlock()
	if old != nil && old != c {
		old.Close()
	}
	for _, fn := range hooks {
		fn(tenantID, agentID)
	}
}

// Detach 仅当仍是当前连接时移除。
func (h *Hub) Detach(tenantID, agentID string, c *Conn) {
	key := connKey(tenantID, agentID)
	h.mu.Lock()
	cur := h.conns[key]
	if cur == c {
		delete(h.conns, key)
	}
	h.mu.Unlock()
}

// Send 向指定代理写一帧；无连接返回 false。
func (h *Hub) Send(tenantID, agentID string, frame protocol.TunnelFrame) bool {
	h.mu.Lock()
	c := h.conns[connKey(tenantID, agentID)]
	h.mu.Unlock()
	if c == nil {
		return false
	}
	return c.Enqueue(frame)
}

// Online 判断代理是否已接入隧道。
func (h *Hub) Online(tenantID, agentID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conns[connKey(tenantID, agentID)] != nil
}

func (h *Hub) dispatch(ctx context.Context, tenantID, agentID string, frame protocol.TunnelFrame) {
	h.mu.Lock()
	fn := h.handlers[frame.Channel]
	h.mu.Unlock()
	if fn == nil {
		return
	}
	fn(ctx, tenantID, agentID, frame)
}

// Conn 是一条代理隧道。
type Conn struct {
	hub      *Hub
	tenantID string
	agentID  string
	ws       *websocket.Conn
	send     chan protocol.TunnelFrame
	cancel   context.CancelFunc
	closed   sync.Once
}

// Serve 读写循环直到断开；调用方须已完成鉴权。
func (h *Hub) Serve(parent context.Context, tenantID, agentID string, ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(parent)
	c := &Conn{
		hub:      h,
		tenantID: tenantID,
		agentID:  agentID,
		ws:       ws,
		send:     make(chan protocol.TunnelFrame, limits.TunnelSendQueue),
		cancel:   cancel,
	}
	h.Attach(tenantID, agentID, c)
	defer func() {
		c.Close()
		h.Detach(tenantID, agentID, c)
	}()

	go c.writeLoop(ctx)
	c.readLoop(ctx)
}

// Enqueue 非阻塞入队；队列满时丢弃最旧 data 帧再试一次，仍满则丢本帧（防泄漏与堵死）。
func (c *Conn) Enqueue(frame protocol.TunnelFrame) bool {
	select {
	case c.send <- frame:
		return true
	default:
	}
	// 挤掉一条排队中的 data，保留 open/close/control。
	select {
	case old := <-c.send:
		if old.Type != protocol.TunnelTypeData {
			select {
			case c.send <- old:
			default:
			}
		}
	default:
	}
	select {
	case c.send <- frame:
		return true
	default:
		return false
	}
}

// Close 关闭写队列与底层连接（幂等）。
func (c *Conn) Close() {
	c.closed.Do(func() {
		c.cancel()
		_ = c.ws.Close(websocket.StatusNormalClosure, "")
	})
}

func (c *Conn) writeLoop(ctx context.Context) {
	ticker := time.NewTicker(limits.TunnelPingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-c.send:
			if err := c.writeFrame(ctx, frame); err != nil {
				logger.Warn(ctx, "tunnel: 写帧失败 agent=%s", c.agentID)
				c.Close()
				return
			}
		case <-ticker.C:
			wctx, cancel := context.WithTimeout(ctx, limits.TunnelWriteWait)
			err := c.ws.Ping(wctx)
			cancel()
			if err != nil {
				c.Close()
				return
			}
		}
	}
}

func (c *Conn) writeFrame(ctx context.Context, frame protocol.TunnelFrame) error {
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if len(raw) > limits.TunnelMaxFrameBytes {
		raw = raw[:limits.TunnelMaxFrameBytes]
	}
	wctx, cancel := context.WithTimeout(ctx, limits.TunnelWriteWait)
	defer cancel()
	return c.ws.Write(wctx, websocket.MessageText, raw)
}

func (c *Conn) readLoop(ctx context.Context) {
	for {
		_, data, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		if len(data) > limits.TunnelMaxFrameBytes {
			continue
		}
		var frame protocol.TunnelFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			continue
		}
		switch frame.Type {
		case protocol.TunnelTypePing:
			_ = c.Enqueue(protocol.TunnelFrame{Type: protocol.TunnelTypePong})
			continue
		case protocol.TunnelTypePong, protocol.TunnelTypeHello:
			continue
		}
		if !protocol.TunnelChannelOK(frame.Channel) {
			continue
		}
		c.hub.dispatch(ctx, c.tenantID, c.agentID, frame)
	}
}
