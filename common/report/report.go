// Package report 把一个 Go 代理采集到的样本按协议上报到中央服务器。
// 它不采集。通讯字段在 protocol。
package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"fluxguard/common/id"
	"fluxguard/common/logger"
	"fluxguard/protocol"
)

const maxBatchItems = 1000

type retryable struct{ err error }

func (e retryable) Error() string { return e.err.Error() }
func (e retryable) Unwrap() error { return e.err }

// Run 按配置间隔采集并上报，直到 ctx 取消。单轮失败只记日志，不退出进程。
func Run(ctx context.Context, cfg Config, agentCode string, collect func(context.Context) (protocol.CollectResult, error)) {
	RunControlled(ctx, cfg, agentCode, collect, nil)
}

// RunControlled 与 Run 相同，并接受命令通道对采集节奏的控制。
func RunControlled(ctx context.Context, cfg Config, agentCode string, collect func(context.Context) (protocol.CollectResult, error), ctl *Control) {
	if ctl == nil {
		ctl = NewControl(cfg.IntervalSec)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	if cfg.SpoolFile == "" {
		cfg.SpoolFile = filepath.Join("data", agentCode+"-spool.jsonl")
	}
	ticker := time.NewTicker(time.Duration(ctl.Seconds()) * time.Second)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := flushSpool(ctx, cfg, client); err != nil {
			logger.Error(ctx, "%s 补报: %v", agentCode, err)
		}
		result, err := collect(ctx)
		if err != nil {
			logger.Error(ctx, "%s 采集: %v", agentCode, err)
		} else if err := Report(ctx, cfg, client, agentCode, result); err != nil {
			logger.Error(ctx, "%s 上报: %v", agentCode, err)
		}
		if err := ctl.Wait(ctx, ticker); err != nil {
			return
		}
	}
}

// Report 把这一轮采集结果上报。超过 1000 条时拆批。网络错误和 5xx 用同一批次号再试一次，仍失败且配置了积压文件时先落盘。
func Report(ctx context.Context, cfg Config, client *http.Client, agentCode string, result protocol.CollectResult) error {
	if !protocol.KnownAgent(agentCode) {
		return fmt.Errorf("代理种类无效")
	}
	if client == nil {
		client = http.DefaultClient
	}
	if len(result.Samples) == 0 && len(result.Events) == 0 && result.Host == nil && result.Java == nil && result.Go == nil && result.AppSide == nil && len(result.AppSides) == 0 {
		return nil
	}
	payload := protocol.Request{
		BatchID:   id.New(),
		AgentCode: agentCode,
		SentAt:    time.Now().UTC(),
		Samples:   result.Samples,
		Events:    result.Events,
		Host:      result.Host,
		Java:      result.Java,
		Go:        result.Go,
		AppSide:   result.AppSide,
		AppSides:  result.AppSides,
	}
	var errs []error
	for _, chunk := range splitRequest(payload) {
		err := postBatch(ctx, client, cfg.ServerURL+protocol.IngestPath, cfg.AgentToken, chunk)
		if err == nil {
			continue
		}
		var again retryable
		if cfg.SpoolFile != "" && errors.As(err, &again) {
			if spoolErr := appendSpool(cfg.SpoolFile, chunk); spoolErr != nil {
				err = errors.Join(err, spoolErr)
			}
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func splitRequest(payload protocol.Request) []protocol.Request {
	if len(payload.Samples) <= maxBatchItems && len(payload.Events) <= maxBatchItems {
		return []protocol.Request{payload}
	}
	samples := payload.Samples
	events := payload.Events
	var out []protocol.Request
	for n := 0; len(samples) > 0 || len(events) > 0; n++ {
		chunk := payload
		chunk.BatchID = chunkID(payload.BatchID, n)
		if n > 0 {
			chunk.Host = nil
			chunk.Java = nil
			chunk.Go = nil
			chunk.AppSide = nil
			chunk.AppSides = nil
		}
		chunk.Samples, samples = takeSamples(samples)
		chunk.Events, events = takeEvents(events)
		out = append(out, chunk)
	}
	return out
}

func takeSamples(in []protocol.Sample) (head, rest []protocol.Sample) {
	if len(in) <= maxBatchItems {
		return in, nil
	}
	return in[:maxBatchItems], in[maxBatchItems:]
}

func takeEvents(in []protocol.Event) (head, rest []protocol.Event) {
	if len(in) <= maxBatchItems {
		return in, nil
	}
	return in[:maxBatchItems], in[maxBatchItems:]
}

func chunkID(base string, n int) string {
	if n == 0 {
		return base
	}
	suffix := "-" + strconv.Itoa(n)
	if len(base)+len(suffix) <= 64 {
		return base + suffix
	}
	return id.New()
}

func postBatch(ctx context.Context, client *http.Client, url, token string, payload any) error {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		last = postOnce(ctx, client, url, token, payload)
		var again retryable
		if last == nil || !errors.As(last, &again) {
			return last
		}
	}
	return last
}

func postOnce(ctx context.Context, client *http.Client, url, token string, payload any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return retryable{err: err}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if protocol.RetryableStatus(resp.StatusCode) {
		return retryable{err: fmt.Errorf("服务端 %d: %s", resp.StatusCode, bytes.TrimSpace(body))}
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("上报被拒绝 %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}
