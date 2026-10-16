package serverupdate

import (
	"fmt"
	"os"
)

// unixScript 在进程退出后换程序。有 systemd 单元或 launchd 清单时走开机服务，否则用 scripts/run。
func unixScript(root, dest, staged string) string {
	backup := backupPath(root)
	lock := lockPath(root)
	return fmt.Sprintf(`#!/bin/sh
ROOT=%q
DEST=%q
STAGED=%q
BACKUP=%q
LOCK=%q
PID=%d
UNIT="/etc/systemd/system/flux-guard-server.service"
PLIST="/Library/LaunchDaemons/flux-guard-server.plist"
LOG="$ROOT/data/server-update.log"
log() { echo "$1" >>"$LOG"; }

sleep 2
if [ -f "$UNIT" ]; then
  systemctl stop flux-guard-server >>"$LOG" 2>&1 || true
elif [ -f "$PLIST" ]; then
  launchctl bootout "system/flux-guard-server" >>"$LOG" 2>&1 || true
fi
i=0
while kill -0 "$PID" 2>/dev/null; do
  i=$((i+1))
  if [ "$i" -gt 150 ]; then
    kill -9 "$PID" 2>/dev/null || true
    break
  fi
  sleep 0.2
done
if [ -f "$DEST" ]; then
  mv "$DEST" "$DEST.bak" || { log "backup binary failed"; exit 1; }
fi
mv "$STAGED" "$DEST" || { log "install binary failed"; exit 1; }
chmod 755 "$DEST" 2>/dev/null || true

start_server() {
  if [ -f "$UNIT" ]; then
    systemctl start flux-guard-server >>"$LOG" 2>&1 || return 1
    sleep 1
    systemctl is-active --quiet flux-guard-server
    return $?
  fi
  if [ -f "$PLIST" ]; then
    launchctl bootstrap system "$PLIST" >>"$LOG" 2>&1 || true
    launchctl kickstart -k "system/flux-guard-server" >>"$LOG" 2>&1 || return 1
    sleep 1
    launchctl print "system/flux-guard-server" 2>/dev/null | grep -q "state = running"
    return $?
  fi
  if [ -x "$ROOT/scripts/run/start.sh" ]; then
    "$ROOT/scripts/run/start.sh" server >>"$LOG" 2>&1
    return $?
  fi
  log "no start script"
  return 1
}

if start_server; then
  rm -f "$DEST.bak"
  rm -rf "$BACKUP" "$LOCK"
  rm -f "$0"
  log "upgraded"
  exit 0
fi
log "start failed; restoring"
if [ -f "$DEST.bak" ]; then
  mv "$DEST.bak" "$DEST" || true
  chmod 755 "$DEST" 2>/dev/null || true
fi
if [ -d "$BACKUP/web-dist" ]; then
  rm -rf "$ROOT/web/dist"
  mkdir -p "$ROOT/web"
  cp -a "$BACKUP/web-dist" "$ROOT/web/dist" || true
fi
if [ -d "$BACKUP/scripts" ]; then
  rm -rf "$ROOT/scripts"
  cp -a "$BACKUP/scripts" "$ROOT/scripts" || true
fi
start_server || true
log "restored"
rm -f "$LOCK" "$0"
exit 1
`, root, dest, staged, backup, lock, os.Getpid())
}

// windowsScript 停掉计划任务或当前进程后换 exe。有同名计划任务时用任务拉起，否则用 start.ps1。
func windowsScript(root, dest, staged string) string {
	backup := backupPath(root)
	lock := lockPath(root)
	return fmt.Sprintf(`@echo off
setlocal
set "ROOT=%s"
set "DEST=%s"
set "STAGED=%s"
set "BACKUP=%s"
set "LOCK=%s"
set "LOG=%%ROOT%%\data\server-update.log"
set "TASK=flux-guard-server"
set "MODE=script"
powershell -NoProfile -Command "Start-Sleep -Seconds 2"
schtasks /Query /TN "%%TASK%%" >NUL 2>&1
if not errorlevel 1 set "MODE=task"
if "%%MODE%%"=="task" schtasks /End /TN "%%TASK%%" >NUL 2>&1
taskkill /F /IM flux-guard-server.exe >NUL 2>&1
powershell -NoProfile -Command "Start-Sleep -Seconds 2"
if exist "%%DEST%%" move /Y "%%DEST%%" "%%DEST%%.bak" >NUL
move /Y "%%STAGED%%" "%%DEST%%" >NUL
if errorlevel 1 goto restore
set "OK=0"
if "%%MODE%%"=="task" (
  schtasks /Run /TN "%%TASK%%" >>"%%LOG%%" 2>&1
  if not errorlevel 1 set "OK=1"
)
if "%%MODE%%"=="script" if exist "%%ROOT%%\scripts\run\start.ps1" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%%ROOT%%\scripts\run\start.ps1" server >>"%%LOG%%" 2>&1
  if not errorlevel 1 set "OK=1"
)
if "%%OK%%"=="0" goto restore
powershell -NoProfile -Command "Start-Sleep -Seconds 2; if (-not (Get-Process -Name flux-guard-server -ErrorAction SilentlyContinue)) { exit 1 }"
if errorlevel 1 goto restore
echo upgraded>>"%%LOG%%"
if exist "%%DEST%%.bak" del /F /Q "%%DEST%%.bak" >NUL 2>&1
if exist "%%BACKUP%%" rmdir /S /Q "%%BACKUP%%" >NUL 2>&1
if exist "%%LOCK%%" del /F /Q "%%LOCK%%" >NUL 2>&1
del "%%~f0" >NUL 2>&1
exit /b 0
:restore
echo start failed; restoring>>"%%LOG%%"
if exist "%%DEST%%.bak" move /Y "%%DEST%%.bak" "%%DEST%%" >NUL
if exist "%%BACKUP%%\web-dist" (
  if exist "%%ROOT%%\web\dist" rmdir /S /Q "%%ROOT%%\web\dist"
  mkdir "%%ROOT%%\web" >NUL 2>&1
  xcopy /E /I /Y "%%BACKUP%%\web-dist" "%%ROOT%%\web\dist\" >NUL
)
if exist "%%BACKUP%%\scripts" (
  if exist "%%ROOT%%\scripts" rmdir /S /Q "%%ROOT%%\scripts"
  xcopy /E /I /Y "%%BACKUP%%\scripts" "%%ROOT%%\scripts\" >NUL
)
if "%%MODE%%"=="task" schtasks /Run /TN "%%TASK%%" >>"%%LOG%%" 2>&1
if "%%MODE%%"=="script" if exist "%%ROOT%%\scripts\run\start.ps1" powershell -NoProfile -ExecutionPolicy Bypass -File "%%ROOT%%\scripts\run\start.ps1" server >>"%%LOG%%" 2>&1
echo restored>>"%%LOG%%"
if exist "%%LOCK%%" del /F /Q "%%LOCK%%" >NUL 2>&1
del "%%~f0" >NUL 2>&1
exit /b 1
`, root, dest, staged, backup, lock)
}
