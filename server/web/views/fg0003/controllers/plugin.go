package controllers

import (
	"fluxguard/server/web/httpx"
	"sort"

	"fluxguard/server/plugin"
	"github.com/gin-gonic/gin"
)

// PluginController 本服务器进程已加载的扩展目录。
type PluginController struct{}

// NewPluginController 创建插件控制器。
func NewPluginController() *PluginController { return &PluginController{} }

// QueryPlugins 返回本进程加载的扩展和平台近期库。
func (c *PluginController) QueryPlugins(g *gin.Context) {
	list := plugin.Infos()
	list = append(list, plugin.Info{
		Code: "recent", Name: "近期样本库", Kind: "SINK", Scope: "SERVER", BuiltinFlag: "Y",
	})
	sort.Slice(list, func(i, j int) bool {
		if list[i].Kind == list[j].Kind {
			return list[i].Code < list[j].Code
		}
		return list[i].Kind < list[j].Kind
	})
	httpx.WriteOK(g, list)
}
