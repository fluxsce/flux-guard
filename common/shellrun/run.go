// Package shellrun 让主机代理经会话隧道为一路终端拉起本机命令行。
// 采集上报和一次性命令不经过这里。
package shellrun

import (
	"context"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"fluxguard/common/agenttunnel"
	"fluxguard/common/logger"
	"fluxguard/protocol"
)

// Bind 把终端通道登记到已有出站隧道客户端（与其它通道共用一条连接）。
func Bind(client *agenttunnel.Client) {
	if client == nil {
		return
	}
	var mu sync.Mutex
	type live struct {
		cancel context.CancelFunc
		gen    uint64
		in     chan protocol.TunnelFrame
	}
	var gen uint64
	lives := map[string]*live{}

	stop := func(sid string) {
		mu.Lock()
		item := lives[sid]
		delete(lives, sid)
		mu.Unlock()
		if item != nil {
			item.cancel()
		}
	}

	client.Handle(protocol.TunnelChannelShell, func(ctx context.Context, send func(protocol.TunnelFrame) error, frame protocol.TunnelFrame) {
		sid := strings.TrimSpace(frame.SessionID)
		if sid == "" {
			return
		}
		switch frame.Type {
		case protocol.TunnelTypeClose:
			stop(sid)
			return
		case protocol.TunnelTypeOpen:
			mu.Lock()
			if old := lives[sid]; old != nil {
				old.cancel()
			}
			gen++
			my := gen
			child, cancel := context.WithCancel(ctx)
			inCh := make(chan protocol.TunnelFrame, 64)
			lives[sid] = &live{cancel: cancel, gen: my, in: inCh}
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
				cols, rows := parseSize(frame.Meta)
				if err := bridge(child, sid, cols, rows, inCh, send); err != nil && child.Err() == nil {
					logger.Error(ctx, "shell: %s: %v", sid, err)
					_ = send(protocol.TunnelFrame{
						Channel:   protocol.TunnelChannelShell,
						SessionID: sid,
						Type:      protocol.TunnelTypeError,
						Data:      "终端异常结束",
					})
				}
			}()
			return
		case protocol.TunnelTypeData, protocol.TunnelTypeControl:
			mu.Lock()
			item := lives[sid]
			mu.Unlock()
			if item == nil {
				return
			}
			select {
			case item.in <- frame:
			default:
				// 输入队列满则丢，避免堵死后内存涨。
			}
		}
	})
}

func parseSize(meta map[string]string) (cols, rows int) {
	cols, rows = 80, 24
	if meta == nil {
		return
	}
	if n, err := strconv.Atoi(strings.TrimSpace(meta["cols"])); err == nil && n > 0 {
		cols = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(meta["rows"])); err == nil && n > 0 {
		rows = n
	}
	return
}

func bridge(ctx context.Context, shellID string, cols, rows int, in <-chan protocol.TunnelFrame, send func(protocol.TunnelFrame) error) error {
	screen, err := openTerm(cols, rows)
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
				data := string(held[:cut])
				if len(data) > 32*1024 {
					data = data[:32*1024]
				}
				if postErr := send(protocol.TunnelFrame{
					Channel:   protocol.TunnelChannelShell,
					SessionID: shellID,
					Type:      protocol.TunnelTypeData,
					Data:      data,
				}); postErr != nil {
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

	curCols, curRows := cols, rows
	for {
		select {
		case <-child.Done():
			return nil
		case frame, ok := <-in:
			if !ok {
				return nil
			}
			switch frame.Type {
			case protocol.TunnelTypeClose:
				return nil
			case protocol.TunnelTypeControl:
				c, r := parseSize(frame.Meta)
				if c > 0 && (c != curCols || r != curRows) {
					if err := screen.Resize(c, r); err != nil && child.Err() == nil {
						logger.Error(ctx, "shell: 调整大小 %s: %v", shellID, err)
					}
					curCols, curRows = c, r
				}
			case protocol.TunnelTypeData:
				if frame.Data == "" {
					continue
				}
				if _, err := io.WriteString(screen, frame.Data); err != nil {
					return err
				}
			}
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
