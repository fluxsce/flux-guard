package tomcat

import (
	"bytes"
	"context"
	"fluxguard/agents/appside/core"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fluxguard/protocol"
)

const startupTailBytes = 2400

func init() {
	core.RegisterMode(tomcatMode{})
}

type tomcatMode struct{}

func (tomcatMode) Name() string { return "tomcat" }

func (tomcatMode) AllowKind(kind string) bool { return kind == "war" }

func (tomcatMode) NeedsRestart(kind string) bool { return true }

func (tomcatMode) SupportUpgrade() bool { return true }

func (tomcatMode) FillSide(side *protocol.AppSide, rt core.Runtime) {
	side.TomcatVersion = TomcatVersion(rt.Base)
}

func (tomcatMode) CollectArtifacts(ctx context.Context, rt core.Runtime) ([]core.Artifact, error) {
	_ = ctx
	found, err := ListWebapps(filepath.Join(rt.Base, "webapps"))
	if err != nil {
		return nil, err
	}
	logs, err := ListRuntimeLogs(filepath.Join(rt.Base, "logs"))
	if err != nil {
		if !core.IsFilesListLimit(err) {
			return nil, err
		}
		// 日志过多时仍上报 WAR，避免整机心跳失败。
		logs = nil
	}
	merged := append(found, logs...)
	if len(merged) > protocol.AppFilesListMax {
		return nil, core.FilesListLimitErr()
	}
	return merged, nil
}

func (tomcatMode) ListInventory(rt core.Runtime) ([]core.Artifact, error) {
	// 与心跳采集同一套 webapps + logs，供控制台刷新同步并注销已删文件。
	return tomcatMode{}.CollectArtifacts(context.Background(), rt)
}

func (m tomcatMode) Replace(rt core.Runtime, call protocol.CommandCall, file string) (core.ApplyReport, error) {
	return ReplaceWar(rt.Base, call.AppName, file, rt.BackupDir, rt.Instance)
}

func (tomcatMode) Upgrade(rt core.Runtime, zipPath string) (core.ApplyReport, error) {
	return core.UpgradeBase(rt.Base, zipPath, rt.BackupDir, rt.Instance)
}

func (tomcatMode) MarkStartupLog(rt core.Runtime) core.LogMark {
	path, err := pickTomcatLog(filepath.Join(strings.TrimSpace(rt.Base), "logs"))
	if err != nil {
		return core.LogMark{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return core.LogMark{Path: path}
	}
	return core.LogMark{Path: path, Offset: info.Size()}
}

func (tomcatMode) ProbeStartup(ctx context.Context, rt core.Runtime, mark core.LogMark) protocol.CommandStep {
	step := protocol.CommandStep{Name: "startup", Status: "done"}
	deadline := time.Now().Add(core.StartupProbeWait(rt, core.TomcatStartupWait))
	logPath := mark.Path
	if logPath == "" {
		if p, err := pickTomcatLog(filepath.Join(strings.TrimSpace(rt.Base), "logs")); err == nil {
			logPath = p
		}
	}
	step.TargetPath = logPath

	for {
		running := core.Running(ctx, rt)
		if logPath == "" {
			if p, err := pickTomcatLog(filepath.Join(strings.TrimSpace(rt.Base), "logs")); err == nil {
				logPath = p
				step.TargetPath = logPath
			}
		}
		var snippet string
		var readErr error
		if logPath != "" {
			snippet, readErr = readLogAfter(logPath, mark.Offset, startupTailBytes)
		}
		if readErr == nil && logPath != "" {
			if earlyReady(snippet) {
				verdict, ok := judgeTomcatStartup(snippet, running)
				step.Detail = formatStartupDetail(verdict, logPath, snippet)
				if !ok {
					step.Status = "failed"
				}
				return step
			}
		}
		if time.Now().After(deadline) {
			if logPath == "" {
				if !running {
					step.Status = "failed"
					if readErr != nil {
						step.Detail = "重启后进程未运行，且未能读取启动日志：" + readErr.Error()
					} else {
						step.Detail = "重启后进程未运行，且未能定位启动日志"
					}
					return step
				}
				step.Detail = "进程已在运行；未能读取启动日志"
				if readErr != nil {
					step.Detail += "：" + readErr.Error()
				}
				return step
			}
			if readErr != nil {
				if !running {
					step.Status = "failed"
					step.Detail = "重启后进程未运行，且未能读取启动日志：" + readErr.Error()
					return step
				}
				step.Detail = "进程已在运行；未能读取启动日志：" + readErr.Error()
				return step
			}
			verdict, ok := judgeTomcatStartup(snippet, running)
			step.Detail = formatStartupDetail(verdict, logPath, snippet)
			if !ok {
				step.Status = "failed"
			}
			return step
		}
		timer := time.NewTimer(core.StartupPollEvery)
		select {
		case <-ctx.Done():
			timer.Stop()
			step.Status = "failed"
			step.Detail = "等待启动时任务已取消"
			return step
		case <-timer.C:
		}
	}
}

func earlyReady(snippet string) bool {
	return catalinaStarted(snippet)
}

// catalinaStarted 判断本次新增日志里 Tomcat 容器是否已写完启动。
// JULI 单行格式在 Server.start() 返回后打出 org.apache.catalina.startup.Catalina.start，
// 这句话的正文随语言变化，类名和方法名不变。堆栈里的 Catalina.start( 不算。
// 没有方法名的格式再退回英文资源句和常见中文译文。
func catalinaStarted(snippet string) bool {
	if snippet == "" {
		return false
	}
	for _, line := range strings.Split(snippet, "\n") {
		if catalinaStartLine(line) {
			return true
		}
	}
	return false
}

func catalinaStartLine(line string) bool {
	const mark = "org.apache.catalina.startup.Catalina.start"
	if i := strings.Index(line, mark); i >= 0 {
		rest := line[i+len(mark):]
		if (rest == "" || rest[0] == ' ' || rest[0] == '\t') && !lineSevere(line) {
			return true
		}
	}
	if strings.Contains(line, "Server startup in") || strings.Contains(strings.ToLower(line), "server startup [") {
		return true
	}
	return strings.Contains(line, "毫秒后服务器启动") || strings.Contains(line, "毫秒内启动")
}

// lineSevere 识别 JULI 的严重级别和端口绑定失败。中文级别名是「严重」，不是 SEVERE。
func lineSevere(line string) bool {
	if strings.Contains(line, "SEVERE:") || strings.Contains(line, "SEVERE [") || strings.Contains(line, "FATAL [") {
		return true
	}
	if strings.Contains(line, " 严重 ") || strings.Contains(line, "严重 [") || strings.Contains(line, "致命 [") {
		return true
	}
	lower := strings.ToLower(line)
	return strings.Contains(lower, "lifecycleexception") ||
		strings.Contains(lower, "address already in use") ||
		strings.Contains(lower, "error listening on") ||
		strings.Contains(line, "BindException") ||
		strings.Contains(line, "地址已在使用")
}

// ReplaceWar 用新包替换 webapps 里同 AppCode 的 WAR（含 order##2.4.1.war），并清掉已展开目录。
func ReplaceWar(base, app, src, backupDir, instance string) (core.ApplyReport, error) {
	app = strings.TrimSpace(app)
	if app == "" || strings.Contains(app, "..") || strings.ContainsAny(app, `/\`) {
		return core.ApplyReport{}, fmt.Errorf("应用名无效")
	}
	webapps := filepath.Join(base, "webapps")
	if err := os.MkdirAll(webapps, 0o755); err != nil {
		return core.ApplyReport{}, err
	}
	dest, label, err := resolveWebappWar(webapps, app)
	if err != nil {
		return core.ApplyReport{}, err
	}
	report := core.ApplyReport{TargetPath: dest}
	parked, err := core.ParkBackup(backupDir, instance, label, dest)
	if err != nil {
		return report, err
	}
	report.BackupPath = parked
	if err := core.CopyFile(src, dest); err != nil {
		_ = core.RestoreBackup(parked, dest)
		return report, err
	}
	exploded := filepath.Join(webapps, app)
	if err := os.RemoveAll(exploded); err != nil {
		return report, err
	}
	return report, nil
}

// resolveWebappWar 定位要覆盖的 WAR：优先已有同 AppCode 文件（含 ## 版本），否则新建 app.war。
func resolveWebappWar(webapps, app string) (dest, label string, err error) {
	exact := filepath.Join(webapps, app+".war")
	if info, statErr := os.Stat(exact); statErr == nil && !info.IsDir() {
		return exact, app + ".war", nil
	}
	entries, readErr := os.ReadDir(webapps)
	if readErr != nil && !os.IsNotExist(readErr) {
		return "", "", readErr
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".war") {
			continue
		}
		name, _ := core.AppCode(entry.Name())
		if name != app {
			continue
		}
		return filepath.Join(webapps, entry.Name()), entry.Name(), nil
	}
	return exact, app + ".war", nil
}

// ListWebapps 读取 webapps 下的 WAR。同名已展开目录不另算一条。
func ListWebapps(dir string) ([]core.Artifact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []core.Artifact
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(name), ".war") {
			continue
		}
		app, version := core.AppCode(name)
		if app == "" || core.BuiltinWars[app] || seen[app] {
			continue
		}
		if err := core.EnsureFilesListCap(len(out)); err != nil {
			return nil, err
		}
		seen[app] = true
		out = append(out, core.Artifact{
			AppName: app,
			Kind:    "war",
			Version: version,
			Path:    filepath.Join(dir, name),
		})
	}
	return out, nil
}

// ListRuntimeLogs 列出运行时 logs 目录下的常见日志文件。
func ListRuntimeLogs(logsDir string) ([]core.Artifact, error) {
	logsDir = strings.TrimSpace(logsDir)
	if logsDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type scored struct {
		art core.Artifact
		mod time.Time
	}
	var list []scored
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		ext := filepath.Ext(name)
		heap := strings.HasPrefix(name, "flux-guard-heap-") && strings.EqualFold(ext, ".hprof")
		if core.FileKind(ext) != "log" && !heap {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		base, _ := core.AppCode(name)
		if base == "" {
			continue
		}
		list = append(list, scored{
			art: core.Artifact{
				AppName: "logs/" + base,
				Kind:    "log",
				Path:    filepath.Join(logsDir, name),
			},
			mod: info.ModTime(),
		})
	}
	if len(list) > protocol.AppFilesListMax {
		return nil, core.FilesListLimitErr()
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].mod.After(list[j].mod)
	})
	out := make([]core.Artifact, 0, len(list))
	for _, item := range list {
		out = append(out, item.art)
	}
	return out, nil
}

// TomcatVersion 从发行说明的第一行读取版本。
func TomcatVersion(base string) string {
	body, err := os.ReadFile(filepath.Join(base, "RELEASE-NOTES"))
	if err != nil {
		return ""
	}
	line := string(body)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	const mark = "Version "
	i := strings.Index(line, mark)
	if i < 0 {
		return ""
	}
	version := strings.TrimSpace(line[i+len(mark):])
	if space := strings.IndexByte(version, ' '); space >= 0 {
		version = version[:space]
	}
	if len(version) > 64 {
		version = version[:64]
	}
	return version
}

func pickTomcatLog(logsDir string) (string, error) {
	preferred := []string{"catalina.out", "catalina.log"}
	for _, name := range preferred {
		full := filepath.Join(logsDir, name)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, nil
		}
	}
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return "", err
	}
	var best string
	var bestMod time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, "catalina") && !strings.HasPrefix(lower, "localhost") {
			continue
		}
		if core.FileKind(filepath.Ext(name)) != "log" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestMod) {
			best = filepath.Join(logsDir, name)
			bestMod = info.ModTime()
		}
	}
	if best == "" {
		return "", fmt.Errorf("logs 下没有 catalina/localhost 日志")
	}
	return best, nil
}

// readLogAfter 只读 sinceOffset 之后的新增内容；日志轮转导致文件变短时读新文件末尾。
func readLogAfter(path string, sinceOffset int64, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		maxBytes = startupTailBytes
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()
	start := sinceOffset
	if start < 0 || start > size {
		// 轮转或截断：只看当前文件末尾。
		start = 0
		if size > int64(maxBytes) {
			start = size - int64(maxBytes)
		}
	} else if size-start > int64(maxBytes) {
		start = size - int64(maxBytes)
	}
	buf := make([]byte, size-start)
	if len(buf) == 0 {
		return "", nil
	}
	if _, err := file.ReadAt(buf, start); err != nil {
		return "", err
	}
	if start > sinceOffset || (sinceOffset == 0 && start > 0) {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 && i+1 < len(buf) {
			buf = buf[i+1:]
		}
	}
	text := string(buf)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.TrimSpace(text), nil
}

func judgeTomcatStartup(snippet string, running bool) (verdict string, ok bool) {
	if strings.TrimSpace(snippet) == "" {
		if running {
			return "进程已在运行；重启后尚无新日志，请人工核对", true
		}
		return "重启后进程未运行，且无新日志", false
	}
	hasOK := catalinaStarted(snippet)
	hasBad := false
	for _, line := range strings.Split(snippet, "\n") {
		if lineSevere(line) {
			hasBad = true
			break
		}
	}

	switch {
	case hasOK && !hasBad:
		return "Tomcat 已启动（Catalina.start 已完成，仅表示容器起来）", true
	case hasOK && hasBad:
		return "容器已启动，但本次日志含严重错误，请核对应用是否部署成功", true
	case hasBad:
		return "启动失败（日志含严重错误或端口绑定失败）", false
	case running:
		return "进程已在运行；日志未见 Catalina.start，请人工核对", true
	default:
		return "重启后进程未运行，且日志未见 Catalina.start", false
	}
}

func formatStartupDetail(verdict, logPath, snippet string) string {
	var b strings.Builder
	b.WriteString(verdict)
	if logPath != "" {
		b.WriteString("\n--- ")
		b.WriteString(filepath.Base(logPath))
		b.WriteString(" ---\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString(snippet)
	out := b.String()
	if len(out) > startupTailBytes+200 {
		out = out[:startupTailBytes+200]
	}
	return out
}
