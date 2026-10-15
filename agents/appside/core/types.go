// Package core 是应用代理的公共类型与调度：Runtime、制品、Mode 登记、备份换包工具。
// 各 mode 子包只依赖本包，不依赖根包 appside，避免循环引用。
package core

// Runtime 是这一进程看守的一套应用。一台机器可以列多套。中央不下发这些路径。
type Runtime struct {
	Kind        string `mapstructure:"kind"`
	Instance    string `mapstructure:"instance"`
	Service     string `mapstructure:"service"`
	Mode        string `mapstructure:"mode"`
	Base        string `mapstructure:"base"`
	Jar         string `mapstructure:"jar"`
	StaticApp   string `mapstructure:"static_app"`
	StaticDir   string `mapstructure:"static_dir"`
	Start       string `mapstructure:"start"`
	Stop        string `mapstructure:"stop"`
	Status      string `mapstructure:"status"`
	Kill        string `mapstructure:"kill"` // 优雅停止仍存活时强杀；Tomcat command 默认按 catalina.base 推导
	// StopGraceSec 优雅 stop 最长秒数，超时后若仍在跑则执行 kill；0 用默认。
	StopGraceSec int `mapstructure:"stop_grace_sec"`
	// StartupWaitSec 启动探测最长等待秒数（轮询日志/进程）；0 用 mode 默认（Tomcat 更长）。
	StartupWaitSec int    `mapstructure:"startup_wait_sec"`
	Description    string `mapstructure:"description"` // 运行时用途描述，上报写入 FG_APP_INST.noteText
	BackupDir      string `mapstructure:"-"`
}

// Artifact 是从一套运行时目录里认出来的一个应用或目录。
type Artifact struct {
	AppName string
	Kind    string
	Version string
	Path    string
}

// ApplyReport 是一次换包/升级落盘结果，供命令回执审计目标与备份路径。
type ApplyReport struct {
	TargetPath string
	BackupPath string
}

// LogMark 是重启前启动日志的字节偏移，避免把历史 SEVERE / Server startup 当成本次结果。
type LogMark struct {
	Path   string
	Offset int64
}
