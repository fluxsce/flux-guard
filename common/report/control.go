package report

import (
	"context"
	"errors"
	"sync"
	"time"

	"fluxguard/protocol"
)

var errInterval = errors.New("上报间隔须在 5 到 3600 秒之间")

// Control 让正在跑的上报循环可以被命令踢一脚，或改掉进程内间隔。
// 间隔只活在这次进程里，重启后回到配置文件。
type Control struct {
	mu    sync.Mutex
	sec   int
	nudge chan struct{}
	reset chan int
}

// NewControl 按当前配置的秒数建一个控制柄。秒数小于等于 0 时按 15 秒。
func NewControl(sec int) *Control {
	if sec <= 0 {
		sec = 15
	}
	return &Control{
		sec:   sec,
		nudge: make(chan struct{}, 1),
		reset: make(chan int, 1),
	}
}

// Seconds 返回当前间隔。
func (c *Control) Seconds() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sec
}

// Nudge 让下一轮立刻采集。采集进行中时，这一脚留到本轮结束。
func (c *Control) Nudge() {
	select {
	case c.nudge <- struct{}{}:
	default:
	}
}

// SetInterval 修改上报间隔。只接受协议规定的范围。
func (c *Control) SetInterval(sec int) error {
	if sec < protocol.IntervalMin || sec > protocol.IntervalMax {
		return errInterval
	}
	c.mu.Lock()
	c.sec = sec
	c.mu.Unlock()
	select {
	case c.reset <- sec:
	default:
		select {
		case <-c.reset:
		default:
		}
		c.reset <- sec
	}
	return nil
}

// Wait 等到到达间隔、被踢一脚、间隔被改掉，或 ctx 取消。
func (c *Control) Wait(ctx context.Context, ticker *time.Ticker) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return nil
	case <-c.nudge:
		return nil
	case sec := <-c.reset:
		ticker.Reset(time.Duration(sec) * time.Second)
		return nil
	}
}
