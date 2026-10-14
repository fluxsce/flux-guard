package agentupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fluxguard/common/buildinfo"
	"fluxguard/protocol"
)

// Apply 下载并安装代理包。成功后安排进程退出，由 helper 完成替换与重启。
// 若当前已是目标版本则直接返回，不重启。
func Apply(ctx context.Context, serverURL, token, agentCode, machineID string, call protocol.CommandCall) (string, error) {
	if !protocol.AgentAction(call.Action) {
		return "", fmt.Errorf("不支持的动作 %s", call.Action)
	}
	if kind := strings.TrimSpace(call.Kind); kind != "" && kind != agentCode {
		return "", fmt.Errorf("安装包种类与本代理不符")
	}
	target := strings.TrimSpace(call.Version)
	if target != "" && target == buildinfo.Version {
		return "已是目标版本 " + target, nil
	}
	if goos := strings.TrimSpace(call.GoOS); goos != "" && !strings.EqualFold(goos, runtime.GOOS) {
		return "", fmt.Errorf("安装包系统为 %s，本机是 %s", goos, runtime.GOOS)
	}
	if goarch := strings.TrimSpace(call.GoArch); goarch != "" && !strings.EqualFold(goarch, runtime.GOARCH) {
		return "", fmt.Errorf("安装包架构为 %s，本机是 %s", goarch, runtime.GOARCH)
	}
	file, err := Download(ctx, serverURL, token, agentCode, machineID, call.ReleaseID, call.SHA256)
	if err != nil {
		return "", err
	}
	defer os.Remove(file)

	root, err := installRoot()
	if err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp("", "fg-agent-stage-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err := untar(file, stage); err != nil {
		return "", err
	}
	payload, err := payloadRoot(stage)
	if err != nil {
		return "", err
	}
	binName := binaryName(agentCode)
	if binName == "" {
		return "", fmt.Errorf("不支持的代理种类")
	}
	newBin := findBinary(payload, binName)
	if newBin == "" {
		return "", fmt.Errorf("安装包里找不到 %s", binName)
	}
	if err := mergeTree(payload, root, binName); err != nil {
		return "", err
	}
	destBin := filepath.Join(root, binName)
	if runtime.GOOS == "windows" {
		destBin += ".exe"
	}
	staged := destBin + ".new"
	if err := copyFile(newBin, staged); err != nil {
		return "", err
	}
	if err := os.Chmod(staged, 0o755); err != nil && runtime.GOOS != "windows" {
		return "", err
	}
	if err := scheduleReplace(root, agentCode, destBin, staged); err != nil {
		return "", err
	}
	go func() {
		time.Sleep(800 * time.Millisecond)
		os.Exit(0)
	}()
	if target == "" {
		return "已接收升级，即将重启", nil
	}
	return "已接收升级到 " + target + "，即将重启", nil
}

func installRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func binaryName(agentCode string) string {
	switch agentCode {
	case protocol.AgentHost:
		return "flux-guard-agent-host"
	case protocol.AgentGo:
		return "flux-guard-agent-goruntime"
	case protocol.AgentAppside:
		return "flux-guard-agent-appside"
	default:
		return ""
	}
}

func runScriptName(agentCode string) string {
	switch agentCode {
	case protocol.AgentHost:
		return "host"
	case protocol.AgentGo:
		return "goruntime"
	case protocol.AgentAppside:
		return "appside"
	default:
		return ""
	}
}

func payloadRoot(stage string) (string, error) {
	entries, err := os.ReadDir(stage)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(stage, e.Name()))
		}
	}
	if len(dirs) == 1 && len(entries) == 1 {
		return dirs[0], nil
	}
	return stage, nil
}

func findBinary(root, base string) string {
	candidates := []string{filepath.Join(root, base), filepath.Join(root, base+".exe")}
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	return ""
}

// mergeTree 把安装包内容合进现有目录。不覆盖已有 configs/*.yaml，只更新 example 与脚本。
func mergeTree(src, dest, binBase string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := filepath.Base(path)
		if base == binBase || base == binBase+".exe" {
			return nil
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if isLiveConfig(rel) {
			if _, err := os.Stat(target); err == nil {
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func isLiveConfig(rel string) bool {
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "configs/") {
		return false
	}
	return strings.HasSuffix(rel, ".yaml") && !strings.HasSuffix(rel, ".yaml.example")
}

func untar(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(root, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode)
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}

func safeJoin(root, name string) (string, error) {
	clean := filepath.Clean(name)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("压缩包路径无效")
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("压缩包路径无效")
	}
	return target, nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
