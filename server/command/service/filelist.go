package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/setting"
	dbx "github.com/fluxsce/dbx/db"
)

// ErrAppsideFilesListLimit 代理返回的清单条数超过 protocol.AppFilesListMax。
var ErrAppsideFilesListLimit = fmt.Errorf("文件数量超过 %d，无法列出", protocol.AppFilesListMax)

// PullAppsideFileList 短拨应用代理，按实例实时列制品目录。
// 响应体在本函数内读完并关闭，不交给调用方。
func (s *Service) PullAppsideFileList(ctx context.Context, tenantID, agentID, instance string) ([]protocol.AppArtifact, error) {
	instance = strings.TrimSpace(instance)
	agentID = strings.TrimSpace(agentID)
	if instance == "" || agentID == "" {
		return nil, fmt.Errorf("参数无效")
	}
	primary, list, addr := s.appsideAddress(ctx, tenantID, agentID)
	if addr == "" {
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
		return nil, fmt.Errorf("代理命令端口不可用，请确认 appside 已配置 command_listen 且在线")
	}
	if !allowedTarget(addr, primary, list) {
		return nil, fmt.Errorf("命令地址不在允许范围")
	}
	base, _, err := dialEndpoint(addr)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimSuffix(base, protocol.ExecPath) + protocol.FilesPath
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("instance", instance)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(token))
	res, err := appsideHTTPClient(60 * time.Second).Do(req)
	if err != nil {
		return nil, fmt.Errorf("拨代理列目录失败: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		if len(msg) == 0 {
			msg = []byte(res.Status)
		}
		return nil, fmt.Errorf("代理拒绝列目录: %s", strings.TrimSpace(string(msg)))
	}
	var out []protocol.AppArtifact
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("列目录响应无效: %w", err)
	}
	if out == nil {
		out = []protocol.AppArtifact{}
	}
	if len(out) > protocol.AppFilesListMax {
		return nil, ErrAppsideFilesListLimit
	}
	return out, nil
}
