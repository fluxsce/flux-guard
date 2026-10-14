// Package apprelease 保存应用更新的制品文件。发布记录在 FG_APP_RELEASE，文件不进库。
package apprelease

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Dir 是制品根目录，按租户和发布号分文件。
const Dir = "data/apprelease"

// Save 把上传内容写到发布目录，并返回校验和。
func Save(tenantID, releaseID string, body io.Reader) (string, error) {
	path, err := Path(tenantID, releaseID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(file, io.TeeReader(body, hash))
	closeErr := file.Close()
	if copyErr != nil {
		os.Remove(path)
		return "", copyErr
	}
	if closeErr != nil {
		os.Remove(path)
		return "", closeErr
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Open 打开已保存的制品。
func Open(tenantID, releaseID string) (*os.File, error) {
	path, err := Path(tenantID, releaseID)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// Path 返回制品路径。租户和发布号只能是安全字符。
func Path(tenantID, releaseID string) (string, error) {
	if !safe(tenantID) || !safe(releaseID) {
		return "", os.ErrInvalid
	}
	return filepath.Join(Dir, tenantID, releaseID), nil
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
