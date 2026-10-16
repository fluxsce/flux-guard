package core

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// LifecycleTimeout 启动类本机命令上限（startup.sh / systemctl start 等通常很快返回）。
// 测试可改短；生产保持默认。
var LifecycleTimeout = 90 * time.Second

// StopGraceTimeout 优雅停止脚本默认上限；超时后若配置了 kill 则强杀。
var StopGraceTimeout = 30 * time.Second

// KillTimeout 强杀命令上限。
var KillTimeout = 15 * time.Second

// StatusTimeout 心跳里查是否在跑的上限，避免 status 脚本卡住整轮采集。
var StatusTimeout = 5 * time.Second

// commandRunner 执行本机固定参数的服务命令。测试替换它，避免真的重启服务。
var commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// shellRunner 执行本机配置里的一整条启动/停止/状态命令。测试替换它。
var shellRunner = func(ctx context.Context, cmdline string) error {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return fmt.Errorf("命令为空")
	}
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", cmdline).Run()
	}
	return exec.CommandContext(ctx, "sh", "-c", cmdline).Run()
}

func withBound(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= d {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

func runShell(ctx context.Context, cmdline string, timeout time.Duration) error {
	cctx, cancel := withBound(ctx, timeout)
	defer cancel()
	err := shellRunner(cctx, cmdline)
	if err != nil && errors.Is(cctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("命令超时（%s）", timeout)
	}
	return err
}

func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	cctx, cancel := withBound(ctx, timeout)
	defer cancel()
	out, err := commandRunner(cctx, name, args...)
	if err != nil && errors.Is(cctx.Err(), context.DeadlineExceeded) {
		return out, fmt.Errorf("命令超时（%s）", timeout)
	}
	return out, err
}

func stopGrace(rt Runtime) time.Duration {
	if rt.StopGraceSec > 0 {
		return time.Duration(rt.StopGraceSec) * time.Second
	}
	return StopGraceTimeout
}

// StartupProbeWait 启动探测最长等待：配置优先，否则用 fallback（Tomcat 等 mode 传入更长默认）。
func StartupProbeWait(rt Runtime, fallback time.Duration) time.Duration {
	if rt.StartupWaitSec > 0 {
		return time.Duration(rt.StartupWaitSec) * time.Second
	}
	if fallback > 0 {
		return fallback
	}
	return StartupWait
}

// Running 查看这一套是否在跑。服务名或命令来自本机配置，不来自中央命令。
// mode=files 是制品目录仓，没有进程，恒为未运行。
func Running(ctx context.Context, rt Runtime) bool {
	if strings.TrimSpace(rt.Mode) == "files" {
		return false
	}
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return false
		}
		_, err := runCommand(ctx, StatusTimeout, "systemctl", "is-active", "--quiet", service)
		return err == nil
	case "windows":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return false
		}
		out, err := runCommand(ctx, StatusTimeout, "sc", "query", service)
		return err == nil && strings.Contains(string(out), "RUNNING")
	case "docker":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return false
		}
		out, err := runCommand(ctx, StatusTimeout, "docker", "inspect", "-f", "{{.State.Running}}", service)
		return err == nil && strings.TrimSpace(string(out)) == "true"
	case "command":
		if alive, ok := tomcatBaseAlive(rt); ok {
			return alive
		}
		return runShell(ctx, rt.Status, StatusTimeout) == nil
	default:
		return false
	}
}

// Start 按本机配置的运行方式启动。files / 未配置启停的静态目录不要调用。
func Start(ctx context.Context, rt Runtime) error {
	if strings.TrimSpace(rt.Mode) == "files" {
		return fmt.Errorf("files 模式无进程启停")
	}
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "systemctl", "start", service); err != nil {
			return fmt.Errorf("启动 %s 失败", service)
		}
		return nil
	case "windows":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "sc", "start", service); err != nil {
			return fmt.Errorf("启动 %s 失败", service)
		}
		return nil
	case "docker":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "docker", "start", service); err != nil {
			return fmt.Errorf("启动 %s 失败", service)
		}
		return nil
	case "command":
		if err := runShell(ctx, rt.Start, LifecycleTimeout); err != nil {
			return fmt.Errorf("启动失败")
		}
		return nil
	default:
		return fmt.Errorf("不认识的运行方式")
	}
}

// stopCommand 有停止脚本时先执行脚本；没有配置 stop 时直接强杀。专供 kind=command。
func stopCommand(ctx context.Context, rt Runtime) error {
	if stop := strings.TrimSpace(rt.Stop); stop != "" {
		_ = runShell(ctx, stop, stopGrace(rt))
		if !Running(ctx, rt) {
			return nil
		}
	}
	if !killTomcatBase(rt) {
		kill := strings.TrimSpace(rt.Kill)
		if kill == "" {
			return fmt.Errorf("停止失败：进程仍在运行（未配置 kill）")
		}
		_ = runShell(ctx, kill, KillTimeout)
	}
	// 给内核一点时间收尸，再查一次。
	timer := time.NewTimer(400 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if !Running(ctx, rt) {
		return nil
	}
	return fmt.Errorf("停止失败：强杀后进程仍在运行")
}

// Stop 按本机配置的运行方式停止。
func Stop(ctx context.Context, rt Runtime) error {
	if strings.TrimSpace(rt.Mode) == "files" {
		return fmt.Errorf("files 模式无进程启停")
	}
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "systemctl", "stop", service); err != nil {
			return fmt.Errorf("停止 %s 失败", service)
		}
		return nil
	case "windows":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "sc", "stop", service); err != nil {
			return fmt.Errorf("停止 %s 失败", service)
		}
		return nil
	case "docker":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "docker", "stop", service); err != nil {
			return fmt.Errorf("停止 %s 失败", service)
		}
		return nil
	case "command":
		return stopCommand(ctx, rt)
	default:
		return fmt.Errorf("不认识的运行方式")
	}
}

// Restart 按本机配置的运行方式重启。静态资源不调用这里。
func Restart(ctx context.Context, rt Runtime) error {
	if strings.TrimSpace(rt.Mode) == "files" {
		return fmt.Errorf("files 模式无进程启停")
	}
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "systemctl", "restart", service); err != nil {
			return fmt.Errorf("重启 %s 失败", service)
		}
		return nil
	case "windows":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		_, _ = runCommand(ctx, LifecycleTimeout, "sc", "stop", service)
		if _, err := runCommand(ctx, LifecycleTimeout, "sc", "start", service); err != nil {
			return fmt.Errorf("重启 %s 失败", service)
		}
		return nil
	case "docker":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := runCommand(ctx, LifecycleTimeout, "docker", "restart", service); err != nil {
			return fmt.Errorf("重启 %s 失败", service)
		}
		return nil
	case "command":
		// 配置或目录里有重启脚本时只走脚本，不先强杀。
		if script := strings.TrimSpace(rt.Restart); script != "" {
			if err := runShell(ctx, script, LifecycleTimeout); err != nil {
				return fmt.Errorf("重启脚本失败")
			}
			return nil
		}
		// 停不干净也尽量 start，避免旧进程占端口时至少把失败写进后续启动探测。
		_ = stopCommand(ctx, rt)
		if err := runShell(ctx, rt.Start, LifecycleTimeout); err != nil {
			return fmt.Errorf("启动失败")
		}
		return nil
	default:
		return fmt.Errorf("不认识的运行方式")
	}
}
