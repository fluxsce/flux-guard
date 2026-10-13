package host

import (
	"testing"
	"time"
)

func TestAdjustLinuxProcIODropsReapedChild(t *testing.T) {
	const childWrite = uint64(15) << 30
	dt := 15 * time.Second
	sec := dt.Seconds()
	prev := &history{proc: map[int32]procCPU{
		10: {ppid: 5, readBytes: 1000, writeBytes: childWrite, hasIO: true},
		5:  {ppid: 1, readBytes: 100, writeBytes: 100, hasIO: true},
		1:  {ppid: 0, readBytes: 1, writeBytes: 1, hasIO: true},
	}}
	cur := &history{proc: map[int32]procCPU{
		5: {hasIO: true},
		1: {hasIO: true},
	}}
	all := []procMem{
		{
			name: "java", pid: 5, hasIO: true,
			readBps:  (1000 + 1000) / sec,
			writeBps: float64(100+childWrite) / sec,
		},
		{
			name: "systemd", pid: 1, hasIO: true,
			readBps:  1000 * 1024 * 1024,
			writeBps: 1000 * 1024 * 1024,
		},
		{
			name: "systemd", pid: 200, hasIO: true,
			writeBps: 800 * 1024 * 1024,
		},
	}
	adjustLinuxProcIO(all, prev, cur, dt)
	if all[0].writeBps < 100/sec-1 || all[0].writeBps > 100/sec+1 {
		t.Fatalf("parent write = %v, want about %v", all[0].writeBps, 100/sec)
	}
	if all[0].readBps < 1000/sec-1 || all[0].readBps > 1000/sec+1 {
		t.Fatalf("parent read = %v, want about %v", all[0].readBps, 1000/sec)
	}
	if all[1].hasIO || all[1].writeBps != 0 {
		t.Fatalf("pid 1 still shown: %+v", all[1])
	}
	if all[2].hasIO || all[2].writeBps != 0 {
		t.Fatalf("user systemd still shown: %+v", all[2])
	}
}
