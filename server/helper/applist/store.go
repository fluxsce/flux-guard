// Package applist 暂存隔离网出站推上来的 files 目录清单（按租户+命令号）。
package applist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"fluxguard/protocol"
)

// Dir 与 appfile 并列，避免混用二进制暂存。
const Dir = "data/applist"

// Save 写入清单 JSON。
func Save(tenantID, jobID string, items []protocol.AppArtifact) error {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if items == nil {
		items = []protocol.AppArtifact{}
	}
	body, err := json.Marshal(items)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Load 读取清单。
func Load(tenantID, jobID string) ([]protocol.AppArtifact, error) {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []protocol.AppArtifact
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []protocol.AppArtifact{}
	}
	return items, nil
}

// Ready 判断清单是否已落盘。
func Ready(tenantID, jobID string) bool {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Remove 删除暂存清单。
func Remove(tenantID, jobID string) {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

// Path 返回暂存路径。
func Path(tenantID, jobID string) (string, error) {
	if !safe(tenantID) || !safe(jobID) {
		return "", os.ErrInvalid
	}
	return filepath.Join(Dir, tenantID, jobID+".json"), nil
}

func safe(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return !strings.Contains(value, "..")
}
