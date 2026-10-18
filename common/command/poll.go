package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fluxguard/common/logger"
	"fluxguard/protocol"
)

func poll(ctx context.Context, cfg Config, exec Exec) {
	client := &http.Client{Timeout: 40 * time.Second}
	for {
		if ctx.Err() != nil {
			return
		}
		call, err := wait(ctx, client, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error(ctx, "command: 取令: %v", err)
			sleep(ctx, 2*time.Second)
			continue
		}
		if call == nil {
			continue
		}
		out, err := exec(ctx, *call)
		result := protocol.CommandResult{
			CommandID: call.CommandID, OK: err == nil, Message: out.Message, Steps: out.Steps,
		}
		if err != nil {
			if result.Message == "" {
				result.Message = err.Error()
			}
		}
		deliverResult(ctx, client, cfg, result)
	}
}

// deliverResult 把回执送到中央。网络失败会重试；4xx 表示命令已结案或被拒绝，不再重试。
func deliverResult(ctx context.Context, client *http.Client, cfg Config, result protocol.CommandResult) {
	wait := 2 * time.Second
	for attempt := 1; attempt <= 6; attempt++ {
		err := postResult(ctx, client, cfg, result)
		if err == nil || ctx.Err() != nil {
			return
		}
		var rejected rejectError
		if errors.As(err, &rejected) {
			logger.Error(ctx, "command: 回执被拒绝: %v", err)
			return
		}
		logger.Error(ctx, "command: 回执第 %d 次失败: %v", attempt, err)
		if attempt == 6 {
			return
		}
		sleep(ctx, wait)
		if wait < 16*time.Second {
			wait *= 2
		}
	}
}

func wait(ctx context.Context, client *http.Client, cfg Config) (*protocol.CommandCall, error) {
	if cfg.MachineID == "" {
		return nil, fmt.Errorf("没有机器编号，无法领取命令")
	}
	endpoint, err := url.Parse(cfg.ServerURL + protocol.WaitPath)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("agentCode", cfg.AgentCode)
	query.Set("machineId", cfg.MachineID)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(cfg.AgentToken))
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("取令 %d", res.StatusCode)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Command *protocol.CommandCall `json:"command"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if !envelope.OK {
		if envelope.Error == "" {
			envelope.Error = "取令失败"
		}
		return nil, errors.New(envelope.Error)
	}
	return envelope.Data.Command, nil
}

func postResult(ctx context.Context, client *http.Client, cfg Config, result protocol.CommandResult) error {
	raw, err := scrubAndMarshal(result)
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(cfg.ServerURL + protocol.ResultPath)
	if err != nil {
		return err
	}
	query := endpoint.Query()
	query.Set("agentCode", cfg.AgentCode)
	query.Set("machineId", cfg.MachineID)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(cfg.AgentToken))
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 400 && res.StatusCode < 500 {
		return rejectError{status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("回执 %d", res.StatusCode)
	}
	return nil
}

// scrubAndMarshal 把回执收成合法 UTF-8 再编码。
// catalina 按本机编码落盘时，日志字节不是 UTF-8，encoding/json 会直接拒绝整包，回执就发不出去，命令一直停在执行中。
func scrubAndMarshal(result protocol.CommandResult) ([]byte, error) {
	result.Message = strings.ToValidUTF8(result.Message, "\uFFFD")
	result.Steps = scrubSteps(result.Steps)
	raw, err := json.Marshal(result)
	if err == nil {
		return raw, nil
	}
	result.Steps = nil
	if result.Message == "" {
		result.Message = "执行结束，日志含无法编码的字符"
	}
	return json.Marshal(result)
}

func scrubSteps(steps []protocol.CommandStep) []protocol.CommandStep {
	for i := range steps {
		steps[i].Name = strings.ToValidUTF8(steps[i].Name, "\uFFFD")
		steps[i].Status = strings.ToValidUTF8(steps[i].Status, "\uFFFD")
		steps[i].TargetPath = strings.ToValidUTF8(steps[i].TargetPath, "\uFFFD")
		steps[i].BackupPath = strings.ToValidUTF8(steps[i].BackupPath, "\uFFFD")
		steps[i].Detail = strings.ToValidUTF8(steps[i].Detail, "\uFFFD")
	}
	return steps
}

type rejectError struct{ status int }

func (e rejectError) Error() string { return fmt.Sprintf("回执 %d", e.status) }

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
