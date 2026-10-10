package plugin

import "testing"

// InstallHost 供外部测试包拉起一份带 yaml 清单的插件进程。
func InstallHost(t *testing.T, root, code, component string, ops []Op) {
	t.Helper()
	installHost(t, root, code, component, ops)
}
