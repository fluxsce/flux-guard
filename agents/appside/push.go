package appside

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fluxguard/protocol"
)

// PushFile 把看守目录内的文件出站推到中央（隔离网代下）。
func PushFile(ctx context.Context, serverURL, agentToken, machineID string, call protocol.CommandCall) error {
	path := strings.TrimSpace(call.TargetPath)
	if path == "" {
		return fmt.Errorf("缺少文件路径")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("不能推送目录")
	}
	name := filepath.Base(path)
	if call.FileName != "" {
		name = filepath.Base(call.FileName)
	}
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		_ = writer.WriteField("commandId", call.CommandID)
		_ = writer.WriteField("machineId", machineID)
		_ = writer.WriteField("agentCode", Code)
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(serverURL), "/") + protocol.FilePushPath)
	if err != nil {
		return err
	}
	q := endpoint.Query()
	q.Set("agentCode", Code)
	q.Set("machineId", machineID)
	endpoint.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), pr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(agentToken))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: 10 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		if len(raw) == 0 {
			return fmt.Errorf("上传失败 %d", res.StatusCode)
		}
		return fmt.Errorf("上传失败: %s", strings.TrimSpace(string(raw)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return nil
}
