package protocol

import "strings"

const (
	// AgentHost 是主机代理。
	AgentHost = "host"
	// AgentApp 是 Go 应用代理。
	AgentApp = "app"
	// AgentJava 是 JVM 代理。
	AgentJava = "java"
)

// KnownAgent 判断种类是否在协议白名单里。大小写必须与常量一致。
func KnownAgent(code string) bool {
	switch strings.TrimSpace(code) {
	case AgentHost, AgentApp, AgentJava:
		return true
	default:
		return false
	}
}
