//go:build unix

package agentupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// scheduleReplace 写旁路脚本：等本进程退出后换二进制，优先 restart，失败则回滚 .bak。
func scheduleReplace(root, agentCode, destBin, staged string) error {
	scriptName := runScriptName(agentCode)
	helper := filepath.Join(root, "data", "agent-update.sh")
	if err := os.MkdirAll(filepath.Dir(helper), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`#!/bin/sh
set -eu
ROOT=%q
DEST=%q
STAGED=%q
NAME=%q
PID=%d
LOG="$ROOT/data/agent-update.log"
i=0
while kill -0 "$PID" 2>/dev/null; do
  i=$((i+1))
  if [ "$i" -gt 60 ]; then
    break
  fi
  sleep 0.2
done
if [ -f "$DEST" ]; then
  mv "$DEST" "$DEST.bak"
fi
mv "$STAGED" "$DEST"
chmod 755 "$DEST" 2>/dev/null || true
ok=0
if [ -x "$ROOT/scripts/run/restart.sh" ]; then
  if "$ROOT/scripts/run/restart.sh" "$NAME" >>"$LOG" 2>&1; then
    ok=1
  fi
elif [ -x "$ROOT/scripts/run/start.sh" ]; then
  if "$ROOT/scripts/run/start.sh" "$NAME" >>"$LOG" 2>&1; then
    ok=1
  fi
fi
if [ "$ok" -ne 1 ]; then
  if [ -f "$DEST.bak" ]; then
    mv "$DEST.bak" "$DEST"
    chmod 755 "$DEST" 2>/dev/null || true
    if [ -x "$ROOT/scripts/run/start.sh" ]; then
      "$ROOT/scripts/run/start.sh" "$NAME" >>"$LOG" 2>&1 || true
    fi
    echo "upgrade failed; restored bak" >>"$LOG"
  fi
  rm -f "$0"
  exit 1
fi
rm -f "$DEST.bak" "$0"
`, root, destBin, staged, scriptName, os.Getpid())
	if err := os.WriteFile(helper, []byte(body), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", helper)
	cmd.Dir = root
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
