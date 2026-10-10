package controllers

import (
	"sort"

	"fluxguard/server/plugin"
	"fluxguard/server/web/httpx"

	"github.com/gin-gonic/gin"
)

// PluginController 本服务器进程已加载的插件目录。
type PluginController struct{}

// NewPluginController 创建插件控制器。
func NewPluginController() *PluginController { return &PluginController{} }

// QueryPlugins 返回本进程已登记的外置插件，带版本、接入组件和进程状态。
func (c *PluginController) QueryPlugins(g *gin.Context) {
	list := plugin.Catalog()
	sort.Slice(list, func(i, j int) bool {
		if list[i].Component != list[j].Component {
			if list[i].Component == "" {
				return false
			}
			if list[j].Component == "" {
				return true
			}
			return list[i].Component < list[j].Component
		}
		if list[i].Name != list[j].Name {
			return list[i].Name < list[j].Name
		}
		return list[i].Code < list[j].Code
	})
	httpx.WriteOK(g, list)
}
