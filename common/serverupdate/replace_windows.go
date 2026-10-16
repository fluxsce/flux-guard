//go:build windows

package serverupdate

import (
	"os"
	"path/filepath"

	"fluxguard/common/agentupdate"
)

// schedule 写下旁路脚本并脱离当前进程启动。
// 计划任务停掉时脚本仍在，可以换 exe 后再用任务或 start.ps1 拉起。
func schedule(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		return err
	}
	helper := filepath.Join(root, "data", "server-update.cmd")
	body := windowsScript(root, binaryPath(root), stagedBinary(root))
	if err := os.WriteFile(helper, []byte(body), 0o644); err != nil {
		return err
	}
	return agentupdate.StartHelper(root, helper, binBase)
}
