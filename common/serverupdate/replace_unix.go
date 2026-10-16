//go:build unix

package serverupdate

import (
	"os"
	"path/filepath"

	"fluxguard/common/agentupdate"
)

// schedule 写下旁路脚本并在开机服务的 cgroup 之外启动。
// 没有 systemd 单元时，StartHelper 用新会话，脚本在 scripts/run 的进程退出后仍能拉起服务。
func schedule(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		return err
	}
	helper := filepath.Join(root, "data", "server-update.sh")
	body := unixScript(root, binaryPath(root), stagedBinary(root))
	if err := os.WriteFile(helper, []byte(body), 0o755); err != nil {
		return err
	}
	return agentupdate.StartHelper(root, helper, binBase)
}
