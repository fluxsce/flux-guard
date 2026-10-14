//go:build windows

package agentupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// scheduleReplace 写旁路脚本：等本进程退出后换二进制，优先 restart，失败则回滚 .bak。
func scheduleReplace(root, agentCode, destBin, staged string) error {
	scriptName := runScriptName(agentCode)
	helper := filepath.Join(root, "data", "agent-update.cmd")
	if err := os.MkdirAll(filepath.Dir(helper), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`@echo off
set ROOT=%s
set DEST=%s
set STAGED=%s
set NAME=%s
set PID=%d
set LOG=%%ROOT%%\data\agent-update.log
:wait
tasklist /FI "PID eq %%PID%%" 2>NUL | find "%%PID%%" >NUL
if not errorlevel 1 (
  timeout /t 1 /nobreak >NUL
  goto wait
)
if exist "%%DEST%%" move /Y "%%DEST%%" "%%DEST%%.bak" >NUL
move /Y "%%STAGED%%" "%%DEST%%" >NUL
set OK=0
if exist "%%ROOT%%\scripts\run\restart.ps1" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%%ROOT%%\scripts\run\restart.ps1" "%%NAME%%" >>"%%LOG%%" 2>&1
  if not errorlevel 1 set OK=1
)
if "%%OK%%"=="0" if exist "%%ROOT%%\scripts\run\start.ps1" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%%ROOT%%\scripts\run\start.ps1" "%%NAME%%" >>"%%LOG%%" 2>&1
  if not errorlevel 1 set OK=1
)
if "%%OK%%"=="0" (
  if exist "%%DEST%%.bak" (
    move /Y "%%DEST%%.bak" "%%DEST%%" >NUL
    if exist "%%ROOT%%\scripts\run\start.ps1" (
      powershell -NoProfile -ExecutionPolicy Bypass -File "%%ROOT%%\scripts\run\start.ps1" "%%NAME%%" >>"%%LOG%%" 2>&1
    )
    echo upgrade failed; restored bak>>"%%LOG%%"
  )
  del "%%~f0" >NUL 2>&1
  exit /b 1
)
if exist "%%DEST%%.bak" del /F /Q "%%DEST%%.bak" >NUL 2>&1
del "%%~f0" >NUL 2>&1
`, root, destBin, staged, scriptName, os.Getpid())
	if err := os.WriteFile(helper, []byte(body), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/C", helper)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200} // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
