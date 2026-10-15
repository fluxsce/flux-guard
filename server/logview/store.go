// Package logview 是应用日志跟读会话：账本 + 进程内 ring，正文不落库。
package logview

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/agenttunnel"
	"fluxguard/server/utils/const/errcode"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/views/fg0008/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	errMissing   = errcode.New("logview.missing", "缺少参数")
	errNotFound  = errcode.New("logview.not_found", "跟读会话不存在")
	errDenied    = errcode.New("logview.denied", "只能查看自己打开的跟读")
	errOffline   = errcode.New("logview.offline", "代理未接入会话隧道")
	errKind      = errcode.New("logview.kind", "该文件不支持在线查看")
	errUserFull  = errcode.New("logview.user.full", "跟读路数已达上限")
	errAgentFull = errcode.New("logview.agent.full", "该代理跟读路数已达上限")
	errClosed    = errcode.New("logview.closed", "跟读已关闭")
)

// View 是控制台打开的一路跟读。
type View struct {
	LogViewID    string `json:"logViewId"`
	AgentID      string `json:"agentId"`
	InstID       string `json:"instId"`
	FileName     string `json:"fileName"`
	ArtifactPath string `json:"-"` // 仅服务端/隧道使用，不返回给浏览器完整敏感盘符时可再裁
}

// Chunk 是控制台尚未读过的输出。
type Chunk struct {
	Text   string `json:"text"`
	Offset int    `json:"offset"`
	Closed bool   `json:"closed"`
}

// Store 管会话与 ring。
type Store struct {
	db   *dbx.DB
	hub  *agenttunnel.Hub
	mu   sync.Mutex
	sess map[string]*session
	seen map[string]time.Time
}

type session struct {
	view     View
	tenantID string
	userID   string
	mu       sync.Mutex
	out      []byte
	base     int
	closed   bool
}

// OpenStore 创建跟读库；注册隧道 apptail 入站处理。
func OpenStore(db *dbx.DB, hub *agenttunnel.Hub) *Store {
	if hub == nil {
		hub = agenttunnel.Global
	}
	s := &Store{
		db:   db,
		hub:  hub,
		sess: map[string]*session{},
		seen: map[string]time.Time{},
	}
	hub.Handle(protocol.TunnelChannelAppTail, s.onTunnelFrame)
	hub.OnAttach(s.onAgentAttach)
	go s.reapLoop()
	return s
}

// onAgentAttach 隧道重连后重推仍打开的跟读，避免断线后代理侧无会话。
func (s *Store) onAgentAttach(tenantID, agentID string) {
	type reopen struct {
		id   string
		path string
	}
	s.mu.Lock()
	var list []reopen
	for _, row := range s.sess {
		if row == nil {
			continue
		}
		row.mu.Lock()
		closed := row.closed
		view := row.view
		row.mu.Unlock()
		if closed || row.tenantID != tenantID || view.AgentID != agentID {
			continue
		}
		list = append(list, reopen{id: view.LogViewID, path: view.ArtifactPath})
	}
	s.mu.Unlock()
	for _, item := range list {
		_ = s.hub.Send(tenantID, agentID, protocol.TunnelFrame{
			Channel:   protocol.TunnelChannelAppTail,
			SessionID: item.id,
			Type:      protocol.TunnelTypeOpen,
			Meta: map[string]string{
				"path":         item.path,
				"initialBytes": itoa(limits.AppTailInitialBytes),
			},
		})
	}
}

var globalStore *Store
var globalOnce sync.Once

// GlobalStore 返回进程内跟读库（须先 InitGlobal）。
func GlobalStore() *Store { return globalStore }

// InitGlobal 初始化全局跟读库。
func InitGlobal(db *dbx.DB) *Store {
	globalOnce.Do(func() {
		globalStore = OpenStore(db, agenttunnel.Global)
	})
	return globalStore
}

func previewable(kind, path string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "log" {
		return true
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "log", "txt", "out":
		return true
	default:
		return false
	}
}

// Open 校验实例并通知代理开始跟读。
func (s *Store) Open(ctx context.Context, tenantID, userID, instID string) (View, error) {
	instID = strings.TrimSpace(instID)
	if tenantID == "" || userID == "" || instID == "" {
		return View{}, errMissing
	}
	inst, err := dao.GetInstance(ctx, s.db, tenantID, instID)
	if err != nil {
		return View{}, err
	}
	path := strings.TrimSpace(inst.ArtifactPath)
	if path == "" || !previewable(inst.ArtifactKind, path) {
		return View{}, errKind
	}
	if !s.hub.Online(tenantID, inst.AgentID) {
		return View{}, errOffline
	}
	s.mu.Lock()
	userN, agentN := 0, 0
	for _, row := range s.sess {
		if row.closed {
			continue
		}
		if row.tenantID == tenantID && row.userID == userID {
			userN++
		}
		if row.tenantID == tenantID && row.view.AgentID == inst.AgentID {
			agentN++
		}
	}
	s.mu.Unlock()
	if userN >= limits.AppTailMaxPerUser {
		return View{}, errUserFull
	}
	if agentN >= limits.AppTailMaxPerAgent {
		return View{}, errAgentFull
	}

	view := View{
		LogViewID:    id.New(),
		AgentID:      inst.AgentID,
		InstID:       inst.InstID,
		FileName:     filepath.Base(path),
		ArtifactPath: path,
	}
	row := &session{view: view, tenantID: tenantID, userID: userID}
	s.mu.Lock()
	s.sess[view.LogViewID] = row
	s.seen[view.LogViewID] = time.Now()
	s.mu.Unlock()

	ok := s.hub.Send(tenantID, inst.AgentID, protocol.TunnelFrame{
		Channel:   protocol.TunnelChannelAppTail,
		SessionID: view.LogViewID,
		Type:      protocol.TunnelTypeOpen,
		Meta: map[string]string{
			"path":         path,
			"initialBytes": itoa(limits.AppTailInitialBytes),
		},
	})
	if !ok {
		s.mu.Lock()
		delete(s.sess, view.LogViewID)
		delete(s.seen, view.LogViewID)
		s.mu.Unlock()
		return View{}, errOffline
	}
	return view, nil
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Touch 保活。
func (s *Store) Touch(logViewID, userID string) error {
	s.mu.Lock()
	row := s.sess[logViewID]
	s.mu.Unlock()
	if row == nil {
		return errNotFound
	}
	if row.userID != userID {
		return errDenied
	}
	s.mu.Lock()
	s.seen[logViewID] = time.Now()
	s.mu.Unlock()
	return nil
}

// Close 关闭并通知代理。
func (s *Store) Close(tenantID, userID, logViewID string) error {
	s.mu.Lock()
	row := s.sess[logViewID]
	s.mu.Unlock()
	if row == nil {
		return errNotFound
	}
	if row.tenantID != tenantID || row.userID != userID {
		return errDenied
	}
	row.mu.Lock()
	row.closed = true
	row.out = nil
	row.mu.Unlock()
	_ = s.hub.Send(tenantID, row.view.AgentID, protocol.TunnelFrame{
		Channel:   protocol.TunnelChannelAppTail,
		SessionID: logViewID,
		Type:      protocol.TunnelTypeClose,
	})
	s.mu.Lock()
	delete(s.sess, logViewID)
	delete(s.seen, logViewID)
	s.mu.Unlock()
	return nil
}

// ReadOutput 读增量；同时 touch。
func (s *Store) ReadOutput(tenantID, userID, logViewID string, offset int) (Chunk, error) {
	s.mu.Lock()
	row := s.sess[logViewID]
	if row != nil {
		s.seen[logViewID] = time.Now()
	}
	s.mu.Unlock()
	if row == nil {
		return Chunk{}, errNotFound
	}
	if row.tenantID != tenantID || row.userID != userID {
		return Chunk{}, errDenied
	}
	row.mu.Lock()
	defer row.mu.Unlock()
	if offset < row.base {
		offset = row.base
	}
	start := offset - row.base
	if start > len(row.out) {
		start = len(row.out)
	}
	chunk := append([]byte(nil), row.out[start:]...)
	return Chunk{
		Text:   string(chunk),
		Offset: row.base + len(row.out),
		Closed: row.closed,
	}, nil
}

func (s *Store) onTunnelFrame(_ context.Context, tenantID, agentID string, frame protocol.TunnelFrame) {
	sid := strings.TrimSpace(frame.SessionID)
	if sid == "" {
		return
	}
	s.mu.Lock()
	row := s.sess[sid]
	s.mu.Unlock()
	if row == nil || row.tenantID != tenantID || row.view.AgentID != agentID {
		return
	}
	switch frame.Type {
	case protocol.TunnelTypeData:
		s.pushOut(row, []byte(frame.Data))
	case protocol.TunnelTypeClose, protocol.TunnelTypeError:
		row.mu.Lock()
		row.closed = true
		row.mu.Unlock()
	}
}

func (s *Store) pushOut(row *session, p []byte) {
	if len(p) == 0 {
		return
	}
	if len(p) > limits.AppTailMaxPush {
		p = p[len(p)-limits.AppTailMaxPush:]
	}
	row.mu.Lock()
	defer row.mu.Unlock()
	if row.closed {
		return
	}
	row.out = append(row.out, p...)
	if len(row.out) > limits.AppTailMaxOut {
		drop := len(row.out) - limits.AppTailMaxOut
		row.base += drop
		row.out = append([]byte(nil), row.out[drop:]...)
	}
}

func (s *Store) reapLoop() {
	t := time.NewTicker(limits.AppTailReapEvery)
	defer t.Stop()
	for range t.C {
		s.reap()
	}
}

func (s *Store) reap() {
	now := time.Now()
	var dead []string
	s.mu.Lock()
	for id, at := range s.seen {
		if now.Sub(at) > limits.AppTailIdle {
			dead = append(dead, id)
		}
	}
	s.mu.Unlock()
	for _, id := range dead {
		s.mu.Lock()
		row := s.sess[id]
		s.mu.Unlock()
		if row == nil {
			continue
		}
		_ = s.Close(row.tenantID, row.userID, id)
	}
}

// PublicView 返回给浏览器的视图（不含绝对路径）。
func PublicView(v View) map[string]string {
	return map[string]string{
		"logViewId": v.LogViewID,
		"agentId":   v.AgentID,
		"instId":    v.InstID,
		"fileName":  v.FileName,
	}
}
