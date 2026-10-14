// Package models 定义命令队列的对外类型与状态常量。
package models

import "errors"

const (
	StatusDialing = "D"
	StatusWaiting = "W"
	StatusRunning = "R"
	StatusDone    = "Y"
	StatusFailed  = "N"

	DeliverPush = "push"
	DeliverPull = "pull"
)

var (
	// ErrUnknownAction 表示动作不在协议名单内。
	ErrUnknownAction = errors.New("不支持的动作")
	// ErrBadInterval 表示上报间隔越界。
	ErrBadInterval = errors.New("上报间隔须在 5 到 3600 秒之间")
	// ErrNotHost 表示目标不是主机代理。
	ErrNotHost = errors.New("当前只支持主机代理")
	// ErrNotAppside 表示目标不是应用侧代理。
	ErrNotAppside = errors.New("这条命令只发给应用侧代理")
	// ErrNotUpdatable 表示该代理种类不支持中央自更新。
	ErrNotUpdatable = errors.New("该代理种类不支持中央升级")
	// ErrBadRelease 表示应用更新或代理自更新缺少发布参数。
	ErrBadRelease = errors.New("发布参数不完整")
	// ErrNotFound 表示找不到代理。
	ErrNotFound = errors.New("找不到这台代理")
	// ErrStopped 表示代理已停用。
	ErrStopped = errors.New("代理已停用")
	// ErrCommandGone 表示命令行已不存在或状态已变。
	ErrCommandGone = errors.New("找不到这条命令")
)

// DispatchInput 是控制台提交的一条命令。
// 应用更新额外带上发布号、应用名、种类、版本、校验和和文件名。
type DispatchInput struct {
	AgentID     string `json:"agentId"`
	Action      string `json:"action"`
	IntervalSec int    `json:"intervalSec"`
	ReleaseID   string `json:"releaseId,omitempty"`
	AppName     string `json:"appName,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Version     string `json:"version,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	Instance    string `json:"instance,omitempty"`
	GoOS        string `json:"goos,omitempty"`
	GoArch      string `json:"goarch,omitempty"`
	TargetPath  string `json:"targetPath,omitempty"`
	BackupPath  string `json:"backupPath,omitempty"`
}

// View 是控制台看到的一行命令。
type View struct {
	CommandID   string `json:"commandId"`
	AgentID     string `json:"agentId"`
	Action      string `json:"action"`
	Status      string `json:"status"`
	DeliverMode string `json:"deliverMode,omitempty"`
	Message     string `json:"message,omitempty"`
	IntervalSec int    `json:"intervalSec,omitempty"`
	AddTime     string `json:"addTime,omitempty"`
	AddWho      string `json:"addWho,omitempty"`
}

// StatusName 把库里的状态码转成控制台用的英文名。
func StatusName(flag string) string {
	switch flag {
	case StatusDialing:
		return "dialing"
	case StatusWaiting:
		return "waiting"
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	case StatusFailed:
		return "failed"
	default:
		return flag
	}
}
