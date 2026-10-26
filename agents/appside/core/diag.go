package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ThreadDumpTimeout 是 jcmd Thread.print 的上限。
var ThreadDumpTimeout = 30 * time.Second

// HeapDumpTimeout 是 jcmd GC.heap_dump 的上限。堆转储会短暂停顿，文件可能很大。
var HeapDumpTimeout = 5 * time.Minute

// threadDumpKeep 写入 logs 的线程堆栈上限。超出则截断并在回执里说明。
const threadDumpKeep = 8 << 20

// diagExcerpt 放进命令步骤的堆栈摘录，避免 noteText 被 16KiB 截断后 JSON 损坏。
const diagExcerpt = 2000

// DiagResult 是一次线程堆栈或堆转储的结果。Path 为空表示没有落文件（例如 SIGQUIT 打进 catalina）。
type DiagResult struct {
	PID     int
	Path    string
	Bytes   int64
	Message string
	Detail  string
}

type jvmProc struct {
	PID int
	Exe string
}

// findJVMs 找出命令行含 arg 的 Java 进程。测试替换它。
var findJVMs = findJVMsOS

// quitProcess 在没有 jcmd 时把线程堆栈打到进程标准输出。Linux 为 SIGQUIT，其它系统不支持。
var quitProcess = func(int) error { return errQuitUnsupported }

var errQuitUnsupported = errors.New("当前系统不能用信号打印线程堆栈")

// ThreadDump 对这一套 Tomcat 打印线程堆栈，写入 base/logs。
func ThreadDump(ctx context.Context, rt Runtime) (DiagResult, error) {
	proc, err := locateTomcat(rt)
	if err != nil {
		return DiagResult{}, err
	}
	out, err := runJcmd(ctx, ThreadDumpTimeout, proc.Exe, strconv.Itoa(proc.PID), "Thread.print", "-l")
	if err != nil && !isNotFound(err) {
		// 老 jcmd 不认 -l 时再打一遍不带锁信息的堆栈。
		plain, plainErr := runJcmd(ctx, ThreadDumpTimeout, proc.Exe, strconv.Itoa(proc.PID), "Thread.print")
		if plainErr == nil {
			out, err = plain, nil
		}
	}
	if isNotFound(err) {
		if qerr := quitProcess(proc.PID); qerr == nil {
			msg := fmt.Sprintf("未找到 jcmd，已向 pid %d 发送 SIGQUIT，线程堆栈打印在 catalina 日志", proc.PID)
			return DiagResult{PID: proc.PID, Message: msg, Detail: msg}, nil
		}
		return DiagResult{}, fmt.Errorf("未找到 jcmd（与该进程 java 同目录或 PATH），无法打印线程堆栈")
	}
	if err != nil {
		return DiagResult{}, fmt.Errorf("打印线程堆栈失败：%s", clipDiag(joinOut(out, err), 400))
	}
	if strings.TrimSpace(string(out)) == "" {
		return DiagResult{}, fmt.Errorf("jcmd 没有输出线程堆栈")
	}
	path, err := diagFile(rt.Base, "flux-guard-threaddump-", ".txt", proc.PID)
	if err != nil {
		return DiagResult{}, err
	}
	body := out
	note := ""
	if len(body) > threadDumpKeep {
		body = body[:threadDumpKeep]
		note = "\n（输出过长，文件只保留前 8MiB）"
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return DiagResult{}, err
	}
	excerpt := clipDiag(string(body), diagExcerpt)
	msg := fmt.Sprintf("已打印线程堆栈 pid %d，见 %s", proc.PID, filepath.Base(path))
	detail := msg + note + "\n" + excerpt
	return DiagResult{PID: proc.PID, Path: path, Bytes: int64(len(body)), Message: msg, Detail: detail}, nil
}

// HeapDump 对这一套 Tomcat 生成可达对象堆转储，写入 base/logs/*.hprof。
func HeapDump(ctx context.Context, rt Runtime) (DiagResult, error) {
	proc, err := locateTomcat(rt)
	if err != nil {
		return DiagResult{}, err
	}
	path, err := diagFile(rt.Base, "flux-guard-heap-", ".hprof", proc.PID)
	if err != nil {
		return DiagResult{}, err
	}
	out, err := runJcmd(ctx, HeapDumpTimeout, proc.Exe, strconv.Itoa(proc.PID), "GC.heap_dump", path)
	if isNotFound(err) {
		return DiagResult{}, fmt.Errorf("未找到 jcmd（与该进程 java 同目录或 PATH），无法生成堆转储")
	}
	if err != nil {
		_ = os.Remove(path)
		return DiagResult{}, fmt.Errorf("生成堆转储失败：%s", clipDiag(joinOut(out, err), 400))
	}
	info, statErr := os.Stat(path)
	if statErr != nil || info.Size() == 0 {
		_ = os.Remove(path)
		extra := clipDiag(string(out), 200)
		if extra == "" {
			extra = "未生成文件"
		}
		return DiagResult{}, fmt.Errorf("生成堆转储失败：%s", extra)
	}
	msg := fmt.Sprintf("已生成堆转储 pid %d，%s，见 %s。只含可达对象，JVM 会短暂停顿；文件很大时请在本机 logs 目录取",
		proc.PID, formatBytes(info.Size()), filepath.Base(path))
	return DiagResult{PID: proc.PID, Path: path, Bytes: info.Size(), Message: msg, Detail: msg}, nil
}

func locateTomcat(rt Runtime) (jvmProc, error) {
	if strings.TrimSpace(rt.Mode) != "tomcat" {
		return jvmProc{}, fmt.Errorf("只有 Tomcat 支持线程堆栈和堆转储")
	}
	arg := catalinaBaseArg(rt.Base)
	if arg == "" {
		return jvmProc{}, fmt.Errorf("tomcat 缺少 base，无法定位进程")
	}
	procs, err := findJVMs(arg)
	if err != nil {
		return jvmProc{}, err
	}
	if len(procs) == 0 {
		return jvmProc{}, fmt.Errorf("这一套 Tomcat 没有在运行")
	}
	if len(procs) > 1 {
		ids := make([]string, 0, len(procs))
		for _, proc := range procs {
			ids = append(ids, strconv.Itoa(proc.PID))
		}
		return jvmProc{}, fmt.Errorf("匹配到多个 Java 进程（pid %s），未执行", strings.Join(ids, ", "))
	}
	return procs[0], nil
}

func diagFile(base, prefix, ext string, pid int) (string, error) {
	logs := filepath.Join(strings.TrimSpace(base), "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return "", err
	}
	name := prefix + time.Now().Format("20060102-150405") + ext
	path := filepath.Join(logs, name)
	if _, err := os.Stat(path); err == nil {
		path = filepath.Join(logs, prefix+time.Now().Format("20060102-150405")+"-"+strconv.Itoa(pid)+ext)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

func runJcmd(ctx context.Context, timeout time.Duration, exe string, args ...string) ([]byte, error) {
	var last error
	for _, bin := range jcmdCandidates(exe) {
		out, err := runCommand(ctx, timeout, bin, args...)
		if err == nil || !isNotFound(err) {
			return out, err
		}
		last = err
	}
	if last == nil {
		last = exec.ErrNotFound
	}
	return nil, last
}

func jcmdCandidates(exe string) []string {
	var bins []string
	exe = strings.TrimSpace(exe)
	if exe != "" {
		name := "jcmd"
		if runtime.GOOS == "windows" {
			name = "jcmd.exe"
		}
		bins = append(bins, filepath.Join(filepath.Dir(exe), name))
	}
	bins = append(bins, "jcmd")
	return bins
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return true
	}
	return strings.Contains(err.Error(), "executable file not found")
}

func joinOut(out []byte, err error) string {
	text := strings.TrimSpace(string(out))
	if err == nil {
		return text
	}
	if text == "" {
		return err.Error()
	}
	return text + " " + err.Error()
}

func clipDiag(s string, n int) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	s = strings.TrimSpace(s)
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func formatBytes(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	v := float64(n)
	unit := "B"
	for _, name := range units {
		v /= 1024
		unit = name
		if v < 1024 {
			break
		}
	}
	return strconv.FormatFloat(v, 'f', 1, 64) + " " + unit
}

// commandLineHasArg 判断一整行命令里是否有与 arg 完全相同的一段（空格或引号分隔）。
func commandLineHasArg(line, arg string) bool {
	if arg == "" || strings.TrimSpace(line) == "" {
		return false
	}
	if line == arg {
		return true
	}
	for _, token := range []string{arg, `"` + arg + `"`, `'` + arg + `'`} {
		if hasBounded(line, token) {
			return true
		}
	}
	return false
}

func hasBounded(line, token string) bool {
	from := 0
	for {
		i := strings.Index(line[from:], token)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(token)
		leftOK := i == 0 || isArgSep(line[i-1])
		rightOK := end == len(line) || isArgSep(line[end])
		if leftOK && rightOK {
			return true
		}
		from = i + len(token)
		if from >= len(line) {
			return false
		}
	}
}

func isArgSep(b byte) bool {
	return b == ' ' || b == '\t' || b == '"' || b == '\''
}

// parseWindowsJava 解析 PowerShell 打出的「pid<TAB>exe<TAB>命令行」。
func parseWindowsJava(text, arg string) []jvmProc {
	self := os.Getpid()
	var out []jvmProc
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		pidText, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		exe, cmd, ok := strings.Cut(rest, "\t")
		if !ok {
			cmd = exe
			exe = ""
		}
		pid, err := strconv.Atoi(strings.TrimSpace(pidText))
		if err != nil || pid <= 1 || pid == self {
			continue
		}
		if !commandLineHasArg(cmd, arg) {
			continue
		}
		out = append(out, jvmProc{PID: pid, Exe: strings.TrimSpace(exe)})
	}
	return out
}

// parseJcmdList 解析 `jcmd -l`：每行「pid 命令行」。
func parseJcmdList(text, arg string) []jvmProc {
	self := os.Getpid()
	var out []jvmProc
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		pidText, rest, ok := strings.Cut(line, " ")
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 1 || pid == self {
			continue
		}
		if !ok {
			rest = ""
		}
		if commandLineHasArg(rest, arg) {
			out = append(out, jvmProc{PID: pid})
		}
	}
	return out
}
