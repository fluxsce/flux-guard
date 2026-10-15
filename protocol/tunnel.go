package protocol

// 代理出站会话隧道：一条 WebSocket 多路复用 shell / apptail 等通道。
// 上报与大文件仍走 HTTPS；本路径只承载交互流，避免按功能堆 REST。

const (
	// TunnelPath 是代理出站连中央的会话隧道（WebSocket）。
	TunnelPath = WebRoot + "/api/agent/v1/tunnel"

	// TunnelChannelShell 是交互终端通道（原 shell/v1 HTTP 三件套）。
	TunnelChannelShell = "shell"
	// TunnelChannelAppTail 是应用日志跟读通道。
	TunnelChannelAppTail = "apptail"

	// 帧类型。payload 含义随 channel + type 变化；禁止携带令牌或口令。
	TunnelTypeHello   = "hello"   // 代理连上后首帧可带 agentCode/machineId（亦可用查询参数）
	TunnelTypePing    = "ping"
	TunnelTypePong    = "pong"
	TunnelTypeOpen    = "open"    // 中央→代理：打开一路会话
	TunnelTypeData    = "data"    // 双向：字节/文本块
	TunnelTypeControl = "control" // 尺寸、偏移、错误摘要等
	TunnelTypeClose   = "close"   // 关闭一路会话
	TunnelTypeError   = "error"   // 稳定短文案，不含堆栈与令牌
)

// TunnelFrame 是隧道上一帧。JSON 文本帧；Data 为 UTF-8 文本或 base64 由通道约定。
// 字段名短小，便于日志脱敏：不得写入 Authorization、token、password。
type TunnelFrame struct {
	Channel   string `json:"channel"`
	SessionID string `json:"sessionId,omitempty"`
	Type      string `json:"type"`
	// Data 是本帧正文。shell 为原始终端字节的 UTF-8 有损文本或原样；apptail 为日志文本。
	Data string `json:"data,omitempty"`
	// Meta 是小控制字段（path、cols、rows、offset、closed 等），值须可公开。
	Meta map[string]string `json:"meta,omitempty"`
}

// TunnelChannelOK 判断通道名是否在允许名单。
func TunnelChannelOK(ch string) bool {
	switch ch {
	case TunnelChannelShell, TunnelChannelAppTail:
		return true
	default:
		return false
	}
}
