package shell

import (
	"context"
	"strconv"
	"strings"

	"fluxguard/protocol"
	"fluxguard/server/agenttunnel"
)

// BindTunnel 把终端会话接到代理出站隧道：去掉 shell/v1 HTTP。
func (s *Store) BindTunnel(hub *agenttunnel.Hub) {
	if s == nil || hub == nil {
		return
	}
	s.hub = hub
	hub.Handle(protocol.TunnelChannelShell, s.onTunnelFrame)
	hub.OnAttach(s.onAgentAttach)
}

func (s *Store) onAgentAttach(tenantID, agentID string) {
	list, err := s.List(context.Background(), tenantID, agentID)
	if err != nil {
		return
	}
	for _, view := range list {
		s.pushOpen(tenantID, agentID, view.ShellID)
	}
}

func (s *Store) onTunnelFrame(ctx context.Context, tenantID, agentID string, frame protocol.TunnelFrame) {
	sid := strings.TrimSpace(frame.SessionID)
	if sid == "" {
		return
	}
	switch frame.Type {
	case protocol.TunnelTypeData:
		_ = s.PutOutput(ctx, tenantID, agentID, sid, frame.Data)
	case protocol.TunnelTypeClose, protocol.TunnelTypeError:
		// 代理侧拆掉 PTY；中央以控制台关闭或空闲回收为准，这里只标记输出结束。
		pipes.shutdown(sid)
	}
}

func (s *Store) pushOpen(tenantID, agentID, shellID string) {
	if s.hub == nil {
		return
	}
	_ = s.hub.Send(tenantID, agentID, protocol.TunnelFrame{
		Channel:   protocol.TunnelChannelShell,
		SessionID: shellID,
		Type:      protocol.TunnelTypeOpen,
		Meta: map[string]string{
			"cols": "80",
			"rows": "24",
		},
	})
}

func (s *Store) pushData(tenantID, agentID, shellID, data string) {
	if s.hub == nil || data == "" {
		return
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	_ = s.hub.Send(tenantID, agentID, protocol.TunnelFrame{
		Channel:   protocol.TunnelChannelShell,
		SessionID: shellID,
		Type:      protocol.TunnelTypeData,
		Data:      data,
	})
}

func (s *Store) pushControl(tenantID, agentID, shellID string, cols, rows int) {
	if s.hub == nil {
		return
	}
	_ = s.hub.Send(tenantID, agentID, protocol.TunnelFrame{
		Channel:   protocol.TunnelChannelShell,
		SessionID: shellID,
		Type:      protocol.TunnelTypeControl,
		Meta: map[string]string{
			"cols": strconv.Itoa(cols),
			"rows": strconv.Itoa(rows),
		},
	})
}

func (s *Store) pushClose(tenantID, agentID, shellID string) {
	if s.hub == nil {
		return
	}
	_ = s.hub.Send(tenantID, agentID, protocol.TunnelFrame{
		Channel:   protocol.TunnelChannelShell,
		SessionID: shellID,
		Type:      protocol.TunnelTypeClose,
	})
}
