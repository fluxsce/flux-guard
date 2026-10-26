//go:build windows

package core

import (
	"context"
	"fmt"
	"time"
)

func findJVMsOS(arg string) ([]jvmProc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	script := "$OutputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false\n" +
		"Get-CimInstance Win32_Process -Filter \"Name = 'java.exe'\" | ForEach-Object {\n" +
		"  $cmd = [string]$_.CommandLine\n" +
		"  $cmd = $cmd -replace '[\\r\\n\\t]', ' '\n" +
		"  $exe = [string]$_.ExecutablePath\n" +
		"  Write-Output ($_.ProcessId.ToString() + [char]9 + $exe + [char]9 + $cmd)\n" +
		"}\n"
	out, err := runCommand(ctx, 20*time.Second, "powershell", "-NoProfile", "-Command", script)
	if err != nil {
		return nil, fmt.Errorf("枚举 java.exe 失败：%s", clipDiag(joinOut(out, err), 200))
	}
	return parseWindowsJava(string(out), arg), nil
}
