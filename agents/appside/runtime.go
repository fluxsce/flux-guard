package appside

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

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

// Running 查看这一套是否在跑。服务名或命令来自本机配置，不来自中央命令。
func Running(ctx context.Context, rt Runtime) bool {
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return false
		}
		_, err := commandRunner(ctx, "systemctl", "is-active", "--quiet", service)
		return err == nil
	case "windows":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return false
		}
		out, err := commandRunner(ctx, "sc", "query", service)
		return err == nil && strings.Contains(string(out), "RUNNING")
	case "docker":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return false
		}
		out, err := commandRunner(ctx, "docker", "inspect", "-f", "{{.State.Running}}", service)
		return err == nil && strings.TrimSpace(string(out)) == "true"
	case "command":
		return shellRunner(ctx, rt.Status) == nil
	default:
		return false
	}
}

// Restart 按本机配置的运行方式重启。静态资源不调用这里。
func Restart(ctx context.Context, rt Runtime) error {
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := commandRunner(ctx, "systemctl", "restart", service); err != nil {
			return fmt.Errorf("重启 %s 失败", service)
		}
		return nil
	case "windows":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		_, _ = commandRunner(ctx, "sc", "stop", service)
		if _, err := commandRunner(ctx, "sc", "start", service); err != nil {
			return fmt.Errorf("重启 %s 失败", service)
		}
		return nil
	case "docker":
		service := strings.TrimSpace(rt.Service)
		if service == "" {
			return fmt.Errorf("未配置服务名")
		}
		if _, err := commandRunner(ctx, "docker", "restart", service); err != nil {
			return fmt.Errorf("重启 %s 失败", service)
		}
		return nil
	case "command":
		_ = shellRunner(ctx, rt.Stop)
		if err := shellRunner(ctx, rt.Start); err != nil {
			return fmt.Errorf("启动失败")
		}
		return nil
	default:
		return fmt.Errorf("不认识的运行方式")
	}
}
