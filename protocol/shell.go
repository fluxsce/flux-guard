package protocol

const (
	// ShellSessionsPath 是代理拉取自己名下仍打开的终端。
	ShellSessionsPath = WebRoot + "/api/shell/v1/sessions"
	// ShellInputPath 是代理长轮询一路终端的输入。
	ShellInputPath = WebRoot + "/api/shell/v1/input"
	// ShellOutputPath 是代理把这路终端的输出送回中央。
	ShellOutputPath = WebRoot + "/api/shell/v1/output"
)
