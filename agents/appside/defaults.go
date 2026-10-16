package appside

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// fillCommandDefaults 在 kind=command 且未写全 start/stop/status 时，按应用类型推导。
// Tomcat 用 base/bin 标准脚本；node / script 用约定启停脚本；jar 在 jar 同目录找约定脚本。
func fillCommandDefaults(rt *Runtime) error {
	if rt == nil || rt.Kind != "command" {
		return nil
	}
	// 文件目录仓只换文件，不启停进程。
	if rt.Mode == "files" {
		return nil
	}
	if rt.Start == "" || rt.Stop == "" || rt.Status == "" {
		start, stop, status, err := defaultCommands(rt)
		if err != nil {
			return err
		}
		if rt.Start == "" {
			rt.Start = start
		}
		if rt.Stop == "" {
			rt.Stop = stop
		}
		if rt.Status == "" {
			rt.Status = status
			rt.StatusAuto = rt.Mode == "tomcat"
		}
	}
	if rt.Mode == "tomcat" && rt.Kill == "" && rt.Base != "" {
		rt.Kill = tomcatKillCommand(rt.Base)
		rt.KillAuto = true
	}
	if rt.Start == "" || rt.Stop == "" || rt.Status == "" {
		return fmt.Errorf("command 模式请填写 start、stop、status，或按约定放置脚本")
	}
	return nil
}

func defaultCommands(rt *Runtime) (start, stop, status string, err error) {
	switch rt.Mode {
	case "tomcat":
		if rt.Base == "" {
			return "", "", "", fmt.Errorf("tomcat 的 command 推导需要 base")
		}
		start, stop, status = tomcatCommands(rt.Base)
		return start, stop, status, nil
	case "node", "script":
		if rt.Base == "" {
			return "", "", "", fmt.Errorf("%s 的 command 推导需要 base", rt.Mode)
		}
		start, stop, status = lifecycleCommands(rt.Base)
		return start, stop, status, nil
	case "jar":
		dir := filepath.Dir(rt.Jar)
		if dir == "" || dir == "." {
			return "", "", "", nil
		}
		start, stop, status = lifecycleCommands(dir)
		return start, stop, status, nil
	default:
		return "", "", "", nil
	}
}

func tomcatCommands(base string) (start, stop, status string) {
	bin := filepath.Join(base, "bin")
	// stock Tomcat 没有 catalina status。Linux 上按完整 base 精确匹配
	// JVM 参数 -Dcatalina.base=<base>，不用实例名，也不走 pgrep 正则。
	status = tomcatStatusCommand(base)
	if runtime.GOOS == "windows" {
		return shellPath(filepath.Join(bin, "startup.bat")),
			shellPath(filepath.Join(bin, "shutdown.bat")),
			status
	}
	return shellPath(filepath.Join(bin, "startup.sh")),
		shellPath(filepath.Join(bin, "shutdown.sh")),
		status
}

// tomcatStatusCommand 是 Windows 上的备用命令。Linux 在 StatusAuto 时直接读进程表，不执行这里。
func tomcatStatusCommand(base string) string {
	marker := catalinaBaseMarker(base)
	if runtime.GOOS == "windows" {
		return tomcatWindowsQuery(marker, false)
	}
	return "flux-guard-tomcat-status " + shellPath(marker)
}

// tomcatKillCommand 是 Windows 上的备用强杀。Linux 在 KillAuto 时直接按同一参数发 SIGKILL。
func tomcatKillCommand(base string) string {
	marker := catalinaBaseMarker(base)
	if runtime.GOOS == "windows" {
		return tomcatWindowsQuery(marker, true)
	}
	return "flux-guard-tomcat-kill " + shellPath(marker)
}

// catalinaBaseMarker 是这一套 Tomcat 在 JVM 命令行里的完整参数，含 base，不含实例名。
func catalinaBaseMarker(base string) string {
	base = filepath.Clean(strings.TrimSpace(base))
	return "-Dcatalina.base=" + base
}

// tomcatWindowsQuery 按完整参数匹配 java.exe。kill 为 true 时结束这些进程，否则只判断是否存在。
func tomcatWindowsQuery(marker string, kill bool) string {
	esc := strings.ReplaceAll(marker, `'`, `''`)
	action := "exit 0"
	if kill {
		action = "Stop-Process -Id $_.ProcessId -Force"
	}
	// 参数两侧必须是开头、空格或引号，避免 /data/app 命中 /data/app-other。
	return `powershell -NoProfile -Command "$m='` + esc + `'; $hit=$false; Get-CimInstance Win32_Process -Filter \"name='java.exe'\" | ForEach-Object { $c=$_.CommandLine; if($c -and (($c -eq $m) -or $c.StartsWith($m+' ') -or $c.Contains(' '+$m+' ') -or $c.EndsWith(' '+$m) -or $c.Contains(' '+$m+'\"') -or $c.StartsWith('\"'+$m))) { $hit=$true; ` + action + ` } }; if($hit){ exit 0 } else { exit 1 }"`
}

// lifecycleCommands 优先找已有约定脚本，没有则落到 base/bin/start|stop|status。
func lifecycleCommands(base string) (start, stop, status string) {
	if found := findLifecycleScripts(base); found[0] != "" {
		return found[0], found[1], found[2]
	}
	bin := filepath.Join(base, "bin")
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}
	return shellPath(filepath.Join(bin, "start"+ext)),
		shellPath(filepath.Join(bin, "stop"+ext)),
		shellPath(filepath.Join(bin, "status"+ext))
}

// findLifecycleScripts 按 base/bin、base、base/scripts 查找 start/stop/status 脚本。
func findLifecycleScripts(base string) [3]string {
	exts := []string{".sh", ".bat", ".cmd"}
	if runtime.GOOS == "windows" {
		exts = []string{".bat", ".cmd", ".sh"}
	}
	dirs := []string{
		filepath.Join(base, "bin"),
		base,
		filepath.Join(base, "scripts"),
	}
	names := []string{"start", "stop", "status"}
	var out [3]string
	for _, dir := range dirs {
		ok := true
		var found [3]string
		for i, name := range names {
			path := firstExisting(dir, name, exts)
			if path == "" {
				ok = false
				break
			}
			found[i] = shellPath(path)
		}
		if ok {
			return found
		}
	}
	return out
}

func firstExisting(dir, stem string, exts []string) string {
	for _, ext := range exts {
		path := filepath.Join(dir, stem+ext)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func shellPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.ContainsAny(path, " \t\"'") {
		return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
	}
	return path
}
