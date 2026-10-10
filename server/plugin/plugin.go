// Package plugin 按 plugins/manifests 的 YAML 清单登记插件进程，并把一次调用的结果交回。
// 清单分节写出挂接、凭据、可执行文件和本机通道。启动方式是 lazy，第一次调用再拉起。
// 同一个插件监听清单里的那条本机管道，每笔调用自己连接，采集和操作可以同时进行。
// 它不解释指标，不写采集表，也不执行维护或文件的业务规则。这些由调用方处理。
// 帧和信封在 protocol。组件口令不写进清单。
package plugin

const (
	// KindService 是独立进程。元数据在 plugins/manifests。
	KindService = "SERVICE"
	// ScopeServer 表示插件由中央进程加载。
	ScopeServer = "SERVER"
	// DirName 是外置插件相对工作目录的文件夹。
	DirName = "plugins"
)

// Info 是注册表里的身份。控制台目录读 Catalog，那里才有版本、组件和进程状态。
type Info struct {
	Code        string `json:"pluginCode"`
	Name        string `json:"pluginName"`
	Kind        string `json:"pluginKind"`
	Scope       string `json:"pluginScope"`
	BuiltinFlag string `json:"builtinFlag"`
}
