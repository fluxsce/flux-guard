// Package shellrun 让主机代理主动连上中央，为一路终端在本机拉起命令行。
// 采集上报和一次性命令不经过这里。
package shellrun

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
	"sync"
	"time"
	"unicode/utf8"

	"fluxguard/common/logger"
	"fluxguard/protocol"
)

// Config 是终端通道配置。令牌与上报、命令通道相同。
type Config struct {
	ServerURL  string
	AgentToken string
	AgentCode  string
	MachineID  string
}

// Run 一直同步打开的终端。中央不可达时只重试，不退出进程。
func Run(ctx context.Context, cfg Config) {
	cfg.ServerURL = strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	cfg.AgentToken = strings.TrimSpace(cfg.AgentToken)
	if cfg.ServerURL == "" || cfg.AgentToken == "" || cfg.MachineID == "" {
		logger.Warn(ctx, "shell: 缺少中央地址、令牌或机器编号，不开启终端")
		return
	}
	client := &http.Client{Timeout: 25 * time.Second}
	var mu sync.Mutex
	type live struct {
		cancel context.CancelFunc
	}
	running := map[string]*live{}
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, item := range running {
			item.cancel()
		}
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		ids, err := listSessions(ctx, client, cfg)
		if err != nil {
			logger.Error(ctx, "shell: 列终端: %v", err)
			sleep(ctx, 2*time.Second)
			continue
		}
		want := map[string]bool{}
		for _, id := range ids {
			want[id] = true
		}
		type started struct {
			id    string
			child context.Context
			slot  *live
		}
		var fresh []started
		mu.Lock()
		for id, item := range running {
			if want[id] {
				continue
			}
			item.cancel()
			delete(running, id)
		}
		for _, id := range ids {
			if _, ok := running[id]; ok {
				continue
			}
			child, cancel := context.WithCancel(ctx)
			slot := &live{cancel: cancel}
			running[id] = slot
			fresh = append(fresh, started{id: id, child: child, slot: slot})
		}
		mu.Unlock()
		for _, item := range fresh {
			go func(shellID string, child context.Context, slot *live) {
				defer func() {
					slot.cancel()
					mu.Lock()
					if running[shellID] == slot {
						delete(running, shellID)
					}
					mu.Unlock()
				}()
				if err := bridge(child, client, cfg, shellID); err != nil && child.Err() == nil {
					logger.Error(ctx, "shell: %s: %v", shellID, err)
				}
			}(item.id, item.child, item.slot)
		}
		sleep(ctx, 2*time.Second)
	}
}

func bridge(ctx context.Context, client *http.Client, cfg Config, shellID string) error {
	screen, err := openTerm(80, 24)
	if err != nil {
		return err
	}
	child, stop := context.WithCancel(ctx)
	defer stop()
	defer screen.Close()
	go func() {
		buf := make([]byte, 8<<10)
		var held []byte
		for {
			n, err := screen.Read(buf)
			held = append(held, buf[:n]...)
			cut := len(held)
			if !utf8.Valid(held) {
				cut = validPrefix(held)
			}
			if cut > 0 {
				if postErr := postOutput(child, client, cfg, shellID, string(held[:cut])); postErr != nil {
					if errors.Is(postErr, errSessionGone) {
						stop()
						return
					}
					if child.Err() != nil {
						return
					}
					logger.Error(ctx, "shell: 回写 %s: %v", shellID, postErr)
				}
				held = append([]byte(nil), held[cut:]...)
			}
			if err != nil {
				stop()
				return
			}
		}
	}()
	cols, rows := 0, 0
	for {
		in, err := waitInput(child, client, cfg, shellID)
		if err != nil {
			return err
		}
		if in.Cols > 0 && (in.Cols != cols || in.Rows != rows) {
			if err := screen.Resize(in.Cols, in.Rows); err != nil && child.Err() == nil {
				logger.Error(ctx, "shell: 调整大小 %s: %v", shellID, err)
			}
			cols, rows = in.Cols, in.Rows
		}
		if in.Data != "" {
			if _, err := io.WriteString(screen, in.Data); err != nil {
				return err
			}
		}
		if in.Closed {
			return nil
		}
	}
}

func validPrefix(p []byte) int {
	i := 0
	for i < len(p) {
		if !utf8.FullRune(p[i:]) {
			return i
		}
		_, size := utf8.DecodeRune(p[i:])
		i += size
	}
	return i
}

type pulled struct {
	Data   string `json:"data"`
	Closed bool   `json:"closed"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

var errSessionGone = errors.New("终端已关闭")

func listSessions(ctx context.Context, client *http.Client, cfg Config) ([]string, error) {
	endpoint, err := withAgent(cfg, protocol.ShellSessionsPath)
	if err != nil {
		return nil, err
	}
	body, status, err := do(ctx, client, http.MethodGet, endpoint, cfg.AgentToken, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("列终端 %d", status)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data []struct {
			ShellID string `json:"shellId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if !envelope.OK {
		return nil, fmt.Errorf("列终端被拒绝")
	}
	ids := make([]string, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		if item.ShellID != "" {
			ids = append(ids, item.ShellID)
		}
	}
	return ids, nil
}

func waitInput(ctx context.Context, client *http.Client, cfg Config, shellID string) (pulled, error) {
	endpoint, err := withAgent(cfg, protocol.ShellInputPath)
	if err != nil {
		return pulled{}, err
	}
	query := endpoint.Query()
	query.Set("shellId", shellID)
	endpoint.RawQuery = query.Encode()
	body, status, err := do(ctx, client, http.MethodGet, endpoint, cfg.AgentToken, nil)
	if err != nil {
		return pulled{}, err
	}
	if status != http.StatusOK {
		return pulled{}, fmt.Errorf("取输入 %d", status)
	}
	var envelope struct {
		OK   bool   `json:"ok"`
		Data pulled `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return pulled{}, err
	}
	if !envelope.OK {
		return pulled{}, fmt.Errorf("取输入被拒绝")
	}
	return envelope.Data, nil
}

func postOutput(ctx context.Context, client *http.Client, cfg Config, shellID, data string) error {
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	endpoint, err := withAgent(cfg, protocol.ShellOutputPath)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"shellId": shellID, "data": data})
	if err != nil {
		return err
	}
	_, status, err := do(ctx, client, http.MethodPost, endpoint, cfg.AgentToken, payload)
	if err != nil {
		return err
	}
	if status == http.StatusBadRequest || status == http.StatusNotFound {
		return errSessionGone
	}
	if status != http.StatusOK {
		return fmt.Errorf("回写 %d", status)
	}
	return nil
}

func withAgent(cfg Config, path string) (*url.URL, error) {
	endpoint, err := url.Parse(cfg.ServerURL + path)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("agentCode", cfg.AgentCode)
	query.Set("machineId", cfg.MachineID)
	endpoint.RawQuery = query.Encode()
	return endpoint, nil
}

func do(ctx context.Context, client *http.Client, method string, endpoint *url.URL, token string, payload []byte) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(token))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return raw, res.StatusCode, err
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
