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
		message, err := exec(ctx, *call)
		result := protocol.CommandResult{CommandID: call.CommandID, OK: err == nil, Message: message}
		if err != nil {
			result.Message = err.Error()
		}
		if err := postResult(ctx, client, cfg, result); err != nil && ctx.Err() == nil {
			logger.Error(ctx, "command: 回执: %v", err)
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
	raw, err := json.Marshal(result)
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
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("回执 %d", res.StatusCode)
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
