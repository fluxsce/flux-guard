// Package agentpack 定位发行包里随中央服务一并交付的代理 tar.gz。
// 路径为安装根下的 agents/flux-guard-agent-*.tar.gz，控制台升级不再要求用户上传。
package agentpack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fluxguard/protocol"
)

// Dir 是相对安装根的代理安装包目录名。
const Dir = "agents"

// FileName 返回种类对应的包装文件名（不含路径）。
func FileName(agentCode string) (string, error) {
	base, err := baseName(agentCode)
	if err != nil {
		return "", err
	}
	return base + ".tar.gz", nil
}

// Open 打开中央 agents/ 目录中对应种类的 tar.gz。调用方负责 Close。
func Open(agentCode string) (*os.File, string, error) {
	name, err := FileName(agentCode)
	if err != nil {
		return nil, "", err
	}
	path, err := locate(name)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("打不开中央安装包 %s：%w", name, err)
	}
	return file, name, nil
}

func baseName(agentCode string) (string, error) {
	switch strings.TrimSpace(agentCode) {
	case protocol.AgentHost:
		return "flux-guard-agent-host", nil
	case protocol.AgentGo:
		return "flux-guard-agent-goruntime", nil
	case protocol.AgentAppside:
		return "flux-guard-agent-appside", nil
	default:
		return "", errors.New("只支持升级 host、go、appside 代理")
	}
}

func locate(fileName string) (string, error) {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		if path := walk(start, fileName); path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("找不到中央 agents/%s（请使用发行包部署，或把构建好的代理包放到安装根下的 agents/）", fileName)
}

func walk(start, fileName string) string {
	dir := start
	for range 8 {
		candidate := filepath.Join(dir, Dir, fileName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
