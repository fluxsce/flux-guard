// Package appfile 暂存「代下」到中央的制品文件。记录不进库，按租户和任务号分文件。
package appfile

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Dir 是代下暂存根目录。
const Dir = "data/appfile"

// Save 把代理推上来的内容写到任务目录。
func Save(tenantID, jobID string, body io.Reader) error {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, body)
	closeErr := file.Close()
	if copyErr != nil {
		os.Remove(path)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(path)
		return closeErr
	}
	return nil
}

// Open 打开已就绪的暂存文件。
func Open(tenantID, jobID string) (*os.File, error) {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// Ready 判断暂存文件是否已存在。
func Ready(tenantID, jobID string) bool {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// Path 返回暂存路径。
func Path(tenantID, jobID string) (string, error) {
	if !safe(tenantID) || !safe(jobID) {
		return "", os.ErrInvalid
	}
	return filepath.Join(Dir, tenantID, jobID), nil
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
