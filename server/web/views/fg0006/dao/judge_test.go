package dao

import "testing"

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

func hasLine(lines []string, text string) bool {
	for _, line := range lines {
		if line == text {
			return true
		}
	}
	return false
}
