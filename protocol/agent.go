package protocol

import "strings"

const (
	// AgentHost 是主机代理。
	AgentHost = "host"
	// AgentGo 是 Go 进程上报自己的运行时。它不是应用管理。
	AgentGo = "go"
	// AgentApp 是 AgentGo 的旧种类名。已经发出的进程仍按这个种类上报。
	AgentApp = "app"
	// AgentJava 是 JVM 代理。
	AgentJava = "java"
	// AgentAppside 是应用代理。它跟一套应用放在同一台机器上，但是独立进程，负责清单和换包。
	AgentAppside = "appside"
)

// KnownAgent 判断种类是否在协议白名单里。大小写必须与常量一致。
func KnownAgent(code string) bool {
	switch strings.TrimSpace(code) {
	case AgentHost, AgentGo, AgentApp, AgentJava, AgentAppside:
		return true
	default:
		return false
	}
}
