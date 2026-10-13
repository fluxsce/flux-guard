package dao

import (
	"strings"
	"testing"
)

func TestJudgeQuietAndShort(t *testing.T) {
	level, lines := judgeWindow(sampleWindow{})
	if level != findingOK || len(lines) != 1 || lines[0] != findingShort {
		t.Fatalf("short %s %+v", level, lines)
	}
	level, lines = judgeWindow(sampleWindow{heapPct: []float64{20, 22, 21}, threads: []float64{10, 11, 10}})
	if level != findingOK || lines[0] != findingQuiet {
		t.Fatalf("quiet %s %+v", level, lines)
	}
}

func TestJudgeOldGenAndPause(t *testing.T) {
	mb := float64(40 << 20)
	level, lines := judgeWindow(sampleWindow{
		heapPct:  []float64{40, 42, 44, 46},
		oldUsed:  []float64{mb, mb * 1.4, mb * 1.8, mb * 2.3},
		oldPause: []float64{10, 12, 80},
	})
	if level != findingAlert || !hasLine(lines, "老年代持续上升，Old 停顿变长") {
		t.Fatalf("%s %+v", level, lines)
	}
}

func TestJudgeHeapDirectAndBlocked(t *testing.T) {
	level, lines := judgeWindow(sampleWindow{
		heapPct:    []float64{90},
		directUsed: []float64{900},
		directMax:  1000,
		threads:    []float64{20},
		hasState:   true,
		blocked:    12,
		runnable:   4,
	})
	if level != findingAlert || !hasLine(lines, "堆使用率已经贴着上限") || !hasLine(lines, "直接内存接近上限") || !hasLine(lines, "有线程堵在锁上") {
		t.Fatalf("%s %+v", level, lines)
	}
}

func TestJudgeBusinessCPU(t *testing.T) {
	level, lines := judgeWindow(sampleWindow{
		heapPct:  []float64{30},
		threads:  []float64{16},
		hasState: true,
		runnable: 10,
		blocked:  0,
		hasCPU:   true,
		cpu:      82,
		hasPause: true,
		pauseMax: 5,
	})
	if level != findingWatch || !hasLine(lines, "进程 CPU 高，线程在跑业务") {
		t.Fatalf("%s %+v", level, lines)
	}
}

func TestJudgeClassAndDirectRise(t *testing.T) {
	meta := float64(64 << 20)
	direct := float64(32 << 20)
	level, lines := judgeWindow(sampleWindow{
		heapPct:    []float64{30, 30, 30, 30},
		metaspace:  []float64{meta, meta * 1.1, meta * 1.2, meta * 1.4},
		loaded:     []float64{1000, 1100, 1300, 1600},
		directUsed: []float64{direct, direct * 1.3, direct * 1.6, direct * 2},
	})
	if level != findingWatch || !hasLine(lines, "元空间和已加载类一起上涨") || !hasLine(lines, "直接内存持续上涨") {
		t.Fatalf("%s %+v", level, lines)
	}
}

func TestJudgeHeapRoundsWithDisplay(t *testing.T) {
	_, lines := judgeWindow(sampleWindow{heapPct: []float64{84.94}, threads: []float64{4}})
	if hasLine(lines, "堆使用率已经贴着上限") {
		t.Fatalf("84.94 stays under the displayed 85.0: %+v", lines)
	}
	_, lines = judgeWindow(sampleWindow{heapPct: []float64{84.95}, threads: []float64{4}})
	if !hasLine(lines, "堆使用率已经贴着上限") {
		t.Fatalf("84.95 displays as 85.0: %+v", lines)
	}
}

func TestJudgeOverheadAndDeadlock(t *testing.T) {
	_, lines := judgeWindow(sampleWindow{
		heapPct: []float64{20}, threads: []float64{4},
		hasOverhead: true, overhead: 9.96,
		hasDeadlock: true, deadlock: 0,
	})
	for _, line := range lines {
		if strings.Contains(line, "停顿开销") || strings.Contains(line, "死锁") {
			t.Fatalf("below threshold %+v", lines)
		}
	}
	_, lines = judgeWindow(sampleWindow{
		heapPct: []float64{20}, threads: []float64{4},
		hasOverhead: true, overhead: 12.34,
		hasDeadlock: true, deadlock: 2,
	})
	if !hasLine(lines, "这一拍停顿开销 12.340%") || !hasLine(lines, "检测到 2 个死锁线程") {
		t.Fatalf("%+v", lines)
	}
}

func TestOldPauseCollector(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: "ZGC Major Cycles", want: false},
		{name: "ZGC Major Pauses", want: true},
		{name: "ZGC Minor Pauses", want: false},
		{name: "G1 Young", want: false},
		{name: "G1 Old", want: true},
		{name: "PS Scavenge", want: false},
		{name: "PS MarkSweep", want: true},
		{name: "", want: false},
	}
	for _, item := range cases {
		if got := oldPauseCollector(item.name); got != item.want {
			t.Fatalf("%s got %v", item.name, got)
		}
	}
}

func hasLine(lines []string, text string) bool {
	for _, line := range lines {
		if line == text {
			return true
		}
	}
	return false
}
