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
		}
	}
	if rt.Mode == "tomcat" && rt.Kill == "" && rt.Base != "" {
		rt.Kill = tomcatKillCommand(rt.Base)
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
	// stock Tomcat 没有 catalina status；用进程命令行里的 catalina.base 判断是否在跑。
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

// tomcatStatusCommand 按 CATALINA_BASE 查 Java 进程。路径含空格时已带引号片段。
func tomcatStatusCommand(base string) string {
	base = filepath.Clean(strings.TrimSpace(base))
	marker := "catalina.base=" + base
	if runtime.GOOS == "windows" {
		// PowerShell：命令行含 catalina.base=<base> 即视为在跑。
		esc := strings.ReplaceAll(marker, `'`, `''`)
		return `powershell -NoProfile -Command "if (Get-CimInstance Win32_Process -Filter \"name='java.exe'\" | Where-Object { $_.CommandLine -like '*` + esc + `*' } | Select-Object -First 1) { exit 0 } else { exit 1 }"`
	}
	// [c] 避免 pgrep 匹配到自身命令行。
	return "pgrep -f " + shellPath("[c]"+marker)
}

// tomcatKillCommand 按 CATALINA_BASE 强杀 Java 进程（优雅 shutdown 超时后用）。
func tomcatKillCommand(base string) string {
	base = filepath.Clean(strings.TrimSpace(base))
	marker := "catalina.base=" + base
	if runtime.GOOS == "windows" {
		esc := strings.ReplaceAll(marker, `'`, `''`)
		return `powershell -NoProfile -Command "Get-CimInstance Win32_Process -Filter \"name='java.exe'\" | Where-Object { $_.CommandLine -like '*` + esc + `*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }"`
	}
	return "pkill -9 -f " + shellPath("[c]"+marker)
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
