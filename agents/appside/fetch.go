package appside

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

// Download 按发布号从中央取回制品，并核对校验和。调用方负责删除返回的临时文件。
func Download(ctx context.Context, serverURL, token, machineID, releaseID, sum string) (string, error) {
	endpoint, err := url.Parse(strings.TrimRight(serverURL, "/") + protocol.ArtifactPath)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("agentCode", Code)
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
		return "", fmt.Errorf("下载制品 %d", res.StatusCode)
	}
	file, err := os.CreateTemp("", "fg-release-*")
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
