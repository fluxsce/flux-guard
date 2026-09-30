package shell

import (
	"context"
	"testing"
	"time"
)

func TestLaneRoundTrip(t *testing.T) {
	l := newLane()
	if !l.pushIn([]byte("ping\n")) {
		t.Fatal("push")
	}
	data, closed, _, _ := l.takeIn(context.Background(), time.Second)
	if closed || string(data) != "ping\n" {
		t.Fatalf("in %q closed %v", data, closed)
	}
	l.pushOut([]byte("pong"))
	text, next, _, skipped := l.readOut(0)
	if text != "pong" || next != 4 || skipped {
		t.Fatalf("out %q %d skipped %v", text, next, skipped)
	}
	text, next, _, skipped = l.readOut(next)
	if text != "" || next != 4 {
		t.Fatalf("tail %q %d", text, next)
	}
}

func TestShutdownReleasesBuffer(t *testing.T) {
	const id = "shell-release"
	l := pipes.lane(id)
	l.pushOut([]byte("screen"))
	pipes.shutdown(id)
	if pipes.get(id) != nil {
		t.Fatal("关闭后缓冲仍留在进程里")
	}
	data, closed, _, _ := l.takeIn(context.Background(), 20*time.Millisecond)
	if !closed || len(data) != 0 {
		t.Fatalf("closed=%v data=%q", closed, data)
	}
}
