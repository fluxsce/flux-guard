package appside

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fluxguard/protocol"
)

// AppTailHandle 处理隧道上下发的日志跟读帧。
// send 不得记录令牌；路径须落在看守目录。
func AppTailHandle(list []Runtime) func(ctx context.Context, send func(protocol.TunnelFrame) error, frame protocol.TunnelFrame) {
	type live struct {
		cancel context.CancelFunc
		gen    uint64
	}
	var mu sync.Mutex
	var gen uint64
	lives := map[string]*live{}
	return func(ctx context.Context, send func(protocol.TunnelFrame) error, frame protocol.TunnelFrame) {
		sid := strings.TrimSpace(frame.SessionID)
		if sid == "" {
			return
		}
		switch frame.Type {
		case protocol.TunnelTypeClose:
			mu.Lock()
			item := lives[sid]
			delete(lives, sid)
			mu.Unlock()
			if item != nil {
				item.cancel()
			}
			return
		case protocol.TunnelTypeOpen:
			path := strings.TrimSpace(frame.Meta["path"])
			initial := 256 << 10
			if v := strings.TrimSpace(frame.Meta["initialBytes"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2<<20 {
					initial = n
				}
			}
			mu.Lock()
			if old := lives[sid]; old != nil {
				old.cancel()
			}
			gen++
			my := gen
			child, cancel := context.WithCancel(ctx)
			lives[sid] = &live{cancel: cancel, gen: my}
			mu.Unlock()
			go func() {
				defer func() {
					mu.Lock()
					if cur := lives[sid]; cur != nil && cur.gen == my {
						delete(lives, sid)
					}
					mu.Unlock()
					cancel()
				}()
				if err := followLog(child, list, path, sid, initial, send); err != nil && child.Err() == nil {
					_ = send(protocol.TunnelFrame{
						Channel:   protocol.TunnelChannelAppTail,
						SessionID: sid,
						Type:      protocol.TunnelTypeError,
						Data:      "无法跟读该文件",
					})
				}
			}()
		}
	}
}

func followLog(ctx context.Context, list []Runtime, path, sid string, initial int, send func(protocol.TunnelFrame) error) error {
	// 看守校验通过后再 Abs 打开，避免未授权路径。
	_, size, body, err := OpenWatchedFile(list, path)
	if err != nil {
		return err
	}
	_ = body.Close()
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	file, err := os.Open(abs)
	if err != nil {
		return err
	}
	defer file.Close()
	if size < 0 {
		if info, e := file.Stat(); e == nil {
			size = info.Size()
		}
	}
	offset := int64(0)
	if size > int64(initial) {
		offset = size - int64(initial)
	}
	if size > offset {
		buf := make([]byte, int(size-offset))
		if _, err := file.ReadAt(buf, offset); err != nil && err != io.EOF {
			return err
		}
		if offset > 0 {
			if i := indexByte(buf, '\n'); i >= 0 && i+1 < len(buf) {
				buf = buf[i+1:]
				offset += int64(i + 1)
			}
		}
		if err := pushChunks(sid, buf, send); err != nil {
			return err
		}
		offset = size
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			st, err := os.Stat(abs)
			if err != nil {
				return err
			}
			cur := st.Size()
			if cur < offset {
				_ = file.Close()
				file, err = os.Open(abs)
				if err != nil {
					return err
				}
				offset = 0
				cur = st.Size()
			}
			if cur == offset {
				continue
			}
			n := cur - offset
			if n > 32<<10 {
				n = 32 << 10
			}
			buf := make([]byte, n)
			nr, err := file.ReadAt(buf, offset)
			if nr > 0 {
				if err := pushChunks(sid, buf[:nr], send); err != nil {
					return err
				}
				offset += int64(nr)
			}
			if err != nil && err != io.EOF {
				return err
			}
		}
	}
}

func pushChunks(sid string, buf []byte, send func(protocol.TunnelFrame) error) error {
	const max = 32 << 10
	for len(buf) > 0 {
		n := len(buf)
		if n > max {
			n = max
		}
		chunk := buf[:n]
		buf = buf[n:]
		if err := send(protocol.TunnelFrame{
			Channel:   protocol.TunnelChannelAppTail,
			SessionID: sid,
			Type:      protocol.TunnelTypeData,
			Data:      string(chunk),
		}); err != nil {
			return err
		}
	}
	return nil
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}
