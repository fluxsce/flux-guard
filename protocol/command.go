package protocol

import "strings"

const (
	// ExecPath 是代理本机命令端口。server 直接拨这台机器，不经过中央根路径。
	ExecPath = "/api/command/v1/exec"
	// WaitPath 是代理向 server 长轮询待取命令。
	WaitPath = WebRoot + "/api/command/v1/wait"
	// ResultPath 是代理把拉取执行的结果写回 server。
	ResultPath = WebRoot + "/api/command/v1/results"

	// ActionCollectNow 让主机代理马上再采一轮。
	ActionCollectNow = "host.collect.now"
	// ActionIntervalSet 修改主机代理进程内的上报间隔，重启后回到配置文件。
	ActionIntervalSet = "host.interval.set"
	// ActionAppReplace 让应用侧代理替换静态资源、WAR 或 JAR。
	ActionAppReplace = "app.file.replace"
	// ActionAppUpgrade 让应用侧代理升级它看守的运行时，并重启该服务。
	ActionAppUpgrade = "app.runtime.upgrade"
	// ActionAgentUpdate 让代理从中央拉取自身安装包并替换后重启。
	// 首次部署仍手工安装；后续版本由中央登记制品并下发本动作。
	ActionAgentUpdate = "agent.self.update"
	// ArtifactPath 是代理按发布号下载制品（应用换包与代理自更新共用）。
	ArtifactPath = WebRoot + "/api/command/v1/artifact"

	// IntervalMin 与 IntervalMax 是上报间隔命令接受的秒数。
	IntervalMin = 5
	IntervalMax = 3600
)

// KnownAction 判断动作是否在允许名单里。名单之外一律拒绝，不接受任意命令行。
func KnownAction(action string) bool {
	switch action {
	case ActionCollectNow, ActionIntervalSet, ActionAppReplace, ActionAppUpgrade, ActionAgentUpdate:
		return true
	default:
		return false
	}
}

// CommandCall 是下发给代理的一条命令。参数只有名单里用得到的字段。
// 应用更新与代理自更新都带 releaseId / sha256 / version；Kind 在自更新时为代理种类。
// GoOS / GoArch 只在 agent.self.update 上有值，代理须与本机 runtime 一致才安装。
type CommandCall struct {
	CommandID   string `json:"commandId"`
	Action      string `json:"action"`
	IntervalSec int    `json:"intervalSec,omitempty"`
	ReleaseID   string `json:"releaseId,omitempty"`
	AppName     string `json:"appName,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Version     string `json:"version,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	Instance    string `json:"instance,omitempty"`
	GoOS        string `json:"goos,omitempty"`
	GoArch      string `json:"goarch,omitempty"`
}

// HostAction 判断动作是否发给主机代理。
func HostAction(action string) bool {
	return action == ActionCollectNow || action == ActionIntervalSet
}

// AppAction 判断动作是否发给应用侧代理。
func AppAction(action string) bool {
	return action == ActionAppReplace || action == ActionAppUpgrade
}

// AgentAction 判断动作是否为代理自身升级。
func AgentAction(action string) bool {
	return action == ActionAgentUpdate
}

// UpdatableAgent 判断种类是否支持中央自更新（Java 代理另议）。
func UpdatableAgent(code string) bool {
	switch strings.TrimSpace(code) {
	case AgentHost, AgentGo, AgentAppside:
		return true
	default:
		return false
	}
}

// ExecReply 是代理命令端口的应答。拨号成功时 server 用它写回同一行。
type ExecReply struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// CommandResult 是拉取执行后代理回写的结果。
type CommandResult struct {
	CommandID string `json:"commandId"`
	OK        bool   `json:"ok"`
	Message   string `json:"message,omitempty"`
}
