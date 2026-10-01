// Package routes 只登记管理端模块和路径前缀。
// 页面在 /fluxguard/，管理接口在 /fluxguard/api/，每个模块再挂自己的目录名，例如 /fluxguard/api/fg0002。
// 登录和模块鉴权在挂载路由时套上，按钮在各模块路由上声明。请求读取在 request，统一响应在 httpx。
package routes

import "fluxguard/protocol"

// WebRoot 控制台页面根路径，也是代理访问中央的根路径。
// 与 protocol.WebRoot 是同一个常量。页面、管理接口、上报、取令、回执和终端都挂在它下面。
const WebRoot = protocol.WebRoot

// APIRoot 管理端 HTTP API 根路径。
const APIRoot = WebRoot + "/api"

// ModuleAPIPrefix 返回模块前缀，例如 /fluxguard/api/fg0021。
func ModuleAPIPrefix(moduleName string) string {
	return APIRoot + "/" + moduleName
}
