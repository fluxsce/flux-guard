package service

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"fluxguard/protocol"
	"fluxguard/server/setting"
	"fluxguard/server/utils/const/limits"
	dbx "github.com/fluxsce/dbx/db"
)

// PullAppsideFile 拨应用代理命令端口，拉取看守目录内的文件流。
// 成功时调用方必须 Close 返回的 body；失败时本函数已关闭响应体。
func (s *Service) PullAppsideFile(ctx context.Context, tenantID, agentID, path string) (name string, body io.ReadCloser, size int64, err error) {
	path = strings.TrimSpace(path)
	agentID = strings.TrimSpace(agentID)
	if path == "" || agentID == "" || strings.Contains(path, "..") {
		return "", nil, 0, fmt.Errorf("参数无效")
	}
	primary, list, addr := s.appsideAddress(ctx, tenantID, agentID)
	if addr == "" {
		// 回落到 FG_AGENT.commandAddress（与列表展示一致）。
		var row dialAddr
		if err := s.db.Get(ctx, &row, `
			SELECT ipAddress, commandAddress
			FROM FG_AGENT WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
			dbx.Args{"tenantId": tenantID, "agentId": agentID}); err == nil {
			primary = strings.TrimSpace(row.IPAddress)
			addr = strings.TrimSpace(row.CommandAddress)
			list = primary
		}
	}
	addr = strings.TrimSpace(addr)
	token := setting.IngestPlain(tenantID)
	if addr == "" || token == "" {
		return "", nil, 0, fmt.Errorf("代理命令端口不可用，请确认 appside 已配置 command_listen 且在线")
	}
	if !allowedTarget(addr, primary, list) {
		return "", nil, 0, fmt.Errorf("命令地址不在允许范围")
	}
	endpoint, _, err := dialFileEndpoint(addr)
	if err != nil {
		return "", nil, 0, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", nil, 0, err
	}
	q := u.Query()
	q.Set("path", path)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, 0, err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(token))
	res, err := appsideHTTPClient(limits.AppFileDownloadTimeout).Do(req)
	if err != nil {
		return "", nil, 0, fmt.Errorf("拨代理下载失败: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		if len(msg) == 0 {
			msg = []byte(res.Status)
		}
		return "", nil, 0, fmt.Errorf("代理拒绝下载: %s", strings.TrimSpace(string(msg)))
	}
	name = filepath.Base(path)
	if cd := res.Header.Get("Content-Disposition"); strings.Contains(cd, "filename=") {
		if i := strings.Index(cd, "filename="); i >= 0 {
			raw := strings.Trim(cd[i+9:], `"' `)
			if raw != "" {
				name = filepath.Base(raw)
			}
		}
	}
	size = res.ContentLength
	return name, res.Body, size, nil
}

func dialFileEndpoint(addr string) (string, net.IP, error) {
	base, ip, err := dialEndpoint(addr)
	if err != nil {
		return "", nil, err
	}
	// dialEndpoint 拼的是 ExecPath，改成 FilePath。
	return strings.TrimSuffix(base, protocol.ExecPath) + protocol.FilePath, ip, nil
}
