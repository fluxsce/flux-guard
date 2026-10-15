package appside

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fluxguard/protocol"
)

// PushFileList 把 files 目录清单出站推到中央（隔离网列目录）。
func PushFileList(ctx context.Context, serverURL, agentToken, machineID string, call protocol.CommandCall, items []protocol.AppArtifact) error {
	commandID := strings.TrimSpace(call.CommandID)
	if commandID == "" {
		return fmt.Errorf("缺少命令")
	}
	if items == nil {
		items = []protocol.AppArtifact{}
	}
	body, err := json.Marshal(map[string]any{
		"commandId": commandID,
		"instance":  strings.TrimSpace(call.Instance),
		"artifacts": items,
	})
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(serverURL), "/") + protocol.FileListPushPath)
	if err != nil {
		return err
	}
	q := endpoint.Query()
	q.Set("agentCode", Code)
	q.Set("machineId", machineID)
	endpoint.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(agentToken))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	client := &http.Client{Timeout: 2 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		if len(raw) == 0 {
			return fmt.Errorf("推送清单失败 %d", res.StatusCode)
		}
		return fmt.Errorf("推送清单失败: %s", strings.TrimSpace(string(raw)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return nil
}
