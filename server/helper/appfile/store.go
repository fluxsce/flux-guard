// Package appfile 暂存「代下」到中央的制品文件。记录不进库，按租户和任务号分文件。
package appfile

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fluxguard/server/utils/const/limits"
)

// ErrTooLarge 表示制品超过 AppFileMaxBytes。
var ErrTooLarge = errors.New("文件超过代下上限（2GiB）")

// Dir 是代下暂存根目录。
const Dir = "data/appfile"

// Progress 是一次代下写入暂存时的字节进度（供控制台轮询）。
type Progress struct {
	BytesReceived int64  `json:"bytesReceived"`
	BytesTotal    int64  `json:"bytesTotal"` // 0 表示未知
	Status        string `json:"status"`     // transferring | ready | failed
	Message       string `json:"message,omitempty"`
	UpdatedAt     int64  `json:"updatedAt,omitempty"`
}

var (
	progressMu  sync.Mutex
	lastPurgeMu sync.Mutex
	lastPurgeAt time.Time
)

// PurgeStale 按 TTL 清理过期暂存；短时内重复调用直接跳过，避免每次代下都扫盘。
func PurgeStale() {
	lastPurgeMu.Lock()
	if time.Since(lastPurgeAt) < 5*time.Minute {
		lastPurgeMu.Unlock()
		return
	}
	lastPurgeAt = time.Now()
	lastPurgeMu.Unlock()
	_, _ = PurgeOlderThan(limits.AppFileStagingTTL)
}

// Save 把代理推上来的内容写到任务目录。
func Save(tenantID, jobID string, body io.Reader) error {
	return SaveWithProgress(tenantID, jobID, body, 0)
}

// SaveWithProgress 写入暂存并周期性刷新进度文件；total 为 0 表示未知总长。
func SaveWithProgress(tenantID, jobID string, body io.Reader, total int64) error {
	if total > limits.AppFileMaxBytes {
		return ErrTooLarge
	}
	path, err := Path(tenantID, jobID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_ = WriteProgress(tenantID, jobID, Progress{
		BytesReceived: 0,
		BytesTotal:    total,
		Status:        "transferring",
	})
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		_ = WriteProgress(tenantID, jobID, Progress{Status: "failed", Message: err.Error(), BytesTotal: total})
		return err
	}
	// 硬顶防止 Content-Length 缺失时无限写入。
	limited := io.LimitReader(body, limits.AppFileMaxBytes+1)
	counter := &byteCounter{total: total, onTick: func(n int64) {
		_ = WriteProgress(tenantID, jobID, Progress{
			BytesReceived: n,
			BytesTotal:    total,
			Status:        "transferring",
		})
	}}
	written, copyErr := io.Copy(file, io.TeeReader(limited, counter))
	closeErr := file.Close()
	if copyErr == nil && written > limits.AppFileMaxBytes {
		os.Remove(path)
		_ = WriteProgress(tenantID, jobID, Progress{
			BytesReceived: counter.n,
			BytesTotal:    total,
			Status:        "failed",
			Message:       ErrTooLarge.Error(),
		})
		return ErrTooLarge
	}
	if copyErr != nil {
		os.Remove(path)
		_ = WriteProgress(tenantID, jobID, Progress{
			BytesReceived: counter.n,
			BytesTotal:    total,
			Status:        "failed",
			Message:       copyErr.Error(),
		})
		return copyErr
	}
	if closeErr != nil {
		os.Remove(path)
		_ = WriteProgress(tenantID, jobID, Progress{
			BytesReceived: counter.n,
			BytesTotal:    total,
			Status:        "failed",
			Message:       closeErr.Error(),
		})
		return closeErr
	}
	_ = WriteProgress(tenantID, jobID, Progress{
		BytesReceived: counter.n,
		BytesTotal:    total,
		Status:        "ready",
	})
	return nil
}

// Open 打开已就绪的暂存文件。
func Open(tenantID, jobID string) (*os.File, error) {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// Ready 判断暂存文件是否已存在。
func Ready(tenantID, jobID string) bool {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// Size 返回暂存文件当前大小；不存在时为 0。
func Size(tenantID, jobID string) int64 {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// Path 返回暂存路径。
func Path(tenantID, jobID string) (string, error) {
	if !safe(tenantID) || !safe(jobID) {
		return "", os.ErrInvalid
	}
	return filepath.Join(Dir, tenantID, jobID), nil
}

// Remove 删除一次代下的暂存与旁路文件。
func Remove(tenantID, jobID string) {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return
	}
	_ = os.Remove(path)
	_ = os.Remove(path + ".name")
	_ = os.Remove(path + ".progress")
}

// PurgeOlderThan 删除修改时间早于 maxAge 的暂存（含失败残留）。
func PurgeOlderThan(maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		maxAge = limits.AppFileStagingTTL
	}
	cutoff := time.Now().Add(-maxAge)
	entries, err := os.ReadDir(Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, tenant := range entries {
		if !tenant.IsDir() {
			continue
		}
		tenantDir := filepath.Join(Dir, tenant.Name())
		files, err := os.ReadDir(tenantDir)
		if err != nil {
			continue
		}
		// 按任务号聚合：去掉 .name / .progress 后缀后去重。
		seen := map[string]struct{}{}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			jobID := strings.TrimSuffix(strings.TrimSuffix(name, ".progress"), ".name")
			if jobID == "" || !safe(jobID) {
				continue
			}
			if _, ok := seen[jobID]; ok {
				continue
			}
			seen[jobID] = struct{}{}
			base := filepath.Join(tenantDir, jobID)
			newest := fileModTime(base)
			for _, side := range []string{base + ".name", base + ".progress"} {
				if t := fileModTime(side); t.After(newest) {
					newest = t
				}
			}
			if newest.IsZero() || newest.After(cutoff) {
				continue
			}
			Remove(tenant.Name(), jobID)
			removed++
		}
	}
	return removed, nil
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// WriteProgress 写入进度旁路文件。
func WriteProgress(tenantID, jobID string, p Progress) error {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = time.Now().Unix()
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	progressMu.Lock()
	defer progressMu.Unlock()
	return os.WriteFile(path+".progress", raw, 0o644)
}

// ReadProgress 读进度；无文件时根据暂存是否就绪推断。
func ReadProgress(tenantID, jobID string) Progress {
	path, err := Path(tenantID, jobID)
	if err != nil {
		return Progress{Status: "failed", Message: "任务无效"}
	}
	progressMu.Lock()
	raw, err := os.ReadFile(path + ".progress")
	progressMu.Unlock()
	if err == nil {
		var p Progress
		if json.Unmarshal(raw, &p) == nil && p.Status != "" {
			// 传输中时用磁盘已写字节校正（进程崩溃后仍可见增长）。
			if p.Status == "transferring" {
				if n := Size(tenantID, jobID); n > p.BytesReceived {
					p.BytesReceived = n
				}
			}
			return p
		}
	}
	if Ready(tenantID, jobID) {
		n := Size(tenantID, jobID)
		return Progress{BytesReceived: n, BytesTotal: n, Status: "ready"}
	}
	return Progress{Status: "transferring"}
}

func safe(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return !strings.Contains(value, "..")
}

// byteCounter 供 TeeReader 计数；大文件按约 1%（夹在 1MiB～16MiB）节流写进度。
type byteCounter struct {
	n      int64
	total  int64
	last   int64
	onTick func(n int64)
}

func (c *byteCounter) Write(p []byte) (int, error) {
	n := len(p)
	c.n += int64(n)
	step := int64(1 << 20) // 1 MiB
	if c.total > 0 {
		if pct := c.total / 100; pct > step {
			step = pct
		}
	}
	if step > 16<<20 {
		step = 16 << 20
	}
	if c.onTick != nil && (c.n-c.last >= step || (c.total > 0 && c.n >= c.total)) {
		c.last = c.n
		c.onTick(c.n)
	}
	return n, nil
}
