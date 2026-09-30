package protocol

const (
	// ExecPath 是代理命令端口。server 拨进来，同一次应答带回结果。
	ExecPath = "/api/command/v1/exec"
	// WaitPath 是代理向 server 长轮询待取命令。
	WaitPath = "/api/command/v1/wait"
	// ResultPath 是代理把拉取执行的结果写回 server。
	ResultPath = "/api/command/v1/results"

	// ActionCollectNow 让主机代理马上再采一轮。
	ActionCollectNow = "host.collect.now"
	// ActionIntervalSet 修改主机代理进程内的上报间隔，重启后回到配置文件。
	ActionIntervalSet = "host.interval.set"

	// IntervalMin 与 IntervalMax 是上报间隔命令接受的秒数。
	IntervalMin = 5
	IntervalMax = 3600
)

// KnownAction 判断动作是否在允许名单里。名单之外一律拒绝，不接受任意命令行。
func KnownAction(action string) bool {
	switch action {
	case ActionCollectNow, ActionIntervalSet:
		return true
	default:
		return false
	}
}

// CommandCall 是下发给代理的一条命令。参数只有名单里用得到的字段。
type CommandCall struct {
	CommandID   string `json:"commandId"`
	Action      string `json:"action"`
	IntervalSec int    `json:"intervalSec,omitempty"`
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
