package protocol

import "strings"

const (
	// ExecPath 是代理本机命令端口。server 直接拨这台机器，不经过中央根路径。
	ExecPath = "/api/command/v1/exec"
	// FilePath 是应用代理本机文件下载。仅允许看守目录内的已登记路径，Bearer 同代理令牌。
	FilePath = "/api/command/v1/file"
	// FilesPath 是应用代理按实例实时列制品目录（files 模式打开文件仓时短拨，不靠心跳登记）。
	FilesPath = "/api/command/v1/files"
	// AppFilesListMax 一套运行时文件仓最多列出的条目数（含目录与日志）。超出则拒绝列出。
	AppFilesListMax = 1000
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
	// ActionAppStart 让应用侧代理按本机配置启动一套运行时。
	ActionAppStart = "app.runtime.start"
	// ActionAppStop 让应用侧代理按本机配置停止一套运行时。
	ActionAppStop = "app.runtime.stop"
	// ActionAppRestart 让应用侧代理按本机配置重启一套运行时（含启动探测）。
	ActionAppRestart = "app.runtime.restart"
	// ActionAppRollback 让应用侧代理把备份目录里的上一份移回目标路径，并按需重启。
	ActionAppRollback = "app.file.rollback"
	// ActionAppFilePush 让应用侧代理把看守目录内的文件出站推到中央（隔离网代下用）。
	ActionAppFilePush = "app.file.push"
	// ActionAppFileList 让应用侧代理扫描 files 目录并把清单出站推到中央（隔离网列目录用）。
	ActionAppFileList = "app.file.list"
	// ActionAgentUpdate 让代理从中央拉取自身安装包并替换后重启。
	// 首次部署仍手工安装；后续版本由中央登记制品并下发本动作。
	ActionAgentUpdate = "agent.self.update"
	// ArtifactPath 是代理按发布号下载制品（应用换包与代理自更新共用）。
	ArtifactPath = WebRoot + "/api/command/v1/artifact"
	// FilePushPath 是代理出站把文件推到中央的路径。
	FilePushPath = WebRoot + "/api/command/v1/file-push"
	// FileListPushPath 是代理出站把 files 目录清单推到中央的路径。
	FileListPushPath = WebRoot + "/api/command/v1/file-list-push"

	// IntervalMin 与 IntervalMax 是上报间隔命令接受的秒数。
	IntervalMin = 5
	IntervalMax = 3600
)

// KnownAction 判断动作是否在允许名单里。名单之外一律拒绝，不接受任意命令行。
func KnownAction(action string) bool {
	switch action {
	case ActionCollectNow, ActionIntervalSet, ActionAppReplace, ActionAppUpgrade,
		ActionAppStart, ActionAppStop, ActionAppRestart,
		ActionAppRollback, ActionAppFilePush, ActionAppFileList, ActionAgentUpdate:
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
	// TargetPath / BackupPath 仅 app.file.rollback：把备份移回目标。
	TargetPath string `json:"targetPath,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
}

// HostAction 判断动作是否发给主机代理。
func HostAction(action string) bool {
	return action == ActionCollectNow || action == ActionIntervalSet
}

// AppAction 判断动作是否发给应用侧代理。
func AppAction(action string) bool {
	return action == ActionAppReplace || action == ActionAppUpgrade ||
		action == ActionAppStart || action == ActionAppStop || action == ActionAppRestart ||
		action == ActionAppRollback || action == ActionAppFilePush || action == ActionAppFileList
}

// FilePushAction 须代理出站执行，中央不拨号（隔离网：代下文件或推目录清单）。
func FilePushAction(action string) bool {
	return action == ActionAppFilePush || action == ActionAppFileList
}

// AppLifecycleAction 启停/重启可能超过短拨总时限（Tomcat shutdown 等），只走长轮询回执。
func AppLifecycleAction(action string) bool {
	return action == ActionAppStart || action == ActionAppStop || action == ActionAppRestart
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

// CommandStep 是一条命令里的可审计步骤（下载、换包路径、重启），不是 CI/CD 流水线。
type CommandStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	TargetPath string `json:"targetPath,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// CommandOutcome 是代理执行结果：短文案给人看，步骤给发布审计。
type CommandOutcome struct {
	Message string        `json:"message,omitempty"`
	Steps   []CommandStep `json:"steps,omitempty"`
}

// ExecReply 是代理命令端口的应答。拨号成功时 server 用它写回同一行。
type ExecReply struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message,omitempty"`
	Steps   []CommandStep `json:"steps,omitempty"`
}

// CommandResult 是拉取执行后代理回写的结果。
type CommandResult struct {
	CommandID string        `json:"commandId"`
	OK        bool          `json:"ok"`
	Message   string        `json:"message,omitempty"`
	Steps     []CommandStep `json:"steps,omitempty"`
}
