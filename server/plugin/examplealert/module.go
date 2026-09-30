// Package examplealert 是服务器加载的菜单扩展。
// 页面地址是 /fluxguard/fg0100，具体页面仍在 web 里按模块号挂载。
package examplealert

import "fluxguard/server/plugin"

func init() {
	plugin.RegisterModule(module{})
}

type module struct{}

func (module) Code() string { return "examplealert" }
func (module) Name() string { return "示例告警" }

// Resources 与 FG_AUTH_RESOURCE.sql 里的 fg0100 种子一致。目录行由脚本写入，启动时不再补。
func (module) Resources() []plugin.ResourceDecl {
	return []plugin.ResourceDecl{
		{
			ResourceID: "fg0100", ResourceName: "示例告警", ResourceCode: "fg0100",
			ResourceType: "MODULE", ResourcePath: "/fg0100", ParentResourceID: "group:observe",
			ResourceLevel: 2, SortOrder: 50,
		},
		{
			ResourceID: "fg0100:view", ResourceName: "查看", ResourceCode: "fg0100:view",
			ResourceType: "BUTTON", ParentResourceID: "fg0100", ResourceLevel: 3, SortOrder: 10,
		},
	}
}
