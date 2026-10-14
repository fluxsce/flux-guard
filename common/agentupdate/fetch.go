// Package agentupdate 让 Go 代理从中央下载自身安装包并替换后重启。
// 中央只下发命令与制品；启停仍由本机 scripts/run 或服务管理完成。
package agentupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"fluxguard/protocol"
)

// Download 按发布号从中央取回安装包，并核对校验和。调用方负责删除返回的临时文件。
func Download(ctx context.Context, serverURL, token, agentCode, machineID, releaseID, sum string) (string, error) {
	endpoint, err := url.Parse(strings.TrimRight(serverURL, "/") + protocol.ArtifactPath)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("agentCode", agentCode)
	query.Set("machineId", machineID)
	query.Set("releaseId", releaseID)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(token))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载安装包 %d", res.StatusCode)
	}
	file, err := os.CreateTemp("", "fg-agent-*")
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(file, io.TeeReader(res.Body, hash)); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(sum)) {
		os.Remove(file.Name())
		return "", fmt.Errorf("校验和不符")
	}
	return file.Name(), nil
}
