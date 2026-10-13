package dao

import (
	"strconv"
	"strings"

	"fluxguard/server/web/views/fg0006/models"
	dbx "github.com/fluxsce/dbx/db"
)

// 这些阈值同时画在曲线上。改这里时要改 web 里 charts.ts 的预警线。
const (
	heapAlertPercent     = 85
	pauseWatchMs         = 200
	pauseAlertMs         = 500
	overheadAlertPercent = 10
)

const (
	findingOK    = "ok"
	findingWatch = "watch"
	findingAlert = "alert"
	findingQuiet = "当前没有明显异常"
	findingShort = "采样还不够，暂时看不出异常"
)

const (
	briefHeapSQL  = `SELECT observedTime, heapUsedPercent, metaspaceUsedBytes FROM FG_JAVA_HEAP WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	briefPoolSQL  = `SELECT observedTime, poolName, usedBytes FROM FG_JAVA_POOL WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND poolName IN ('old', 'direct') ORDER BY observedTime DESC, pointId DESC`
	briefMaxSQL   = `SELECT observedTime, maxBytes FROM FG_JAVA_POOL_MAX WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND poolName = 'direct' ORDER BY observedTime DESC, pointId DESC`
	briefRunSQL   = `SELECT observedTime, processCpuPercent, loadedClassCount, pauseOverheadPercent FROM FG_JAVA_RUNTIME WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	briefThrSQL   = `SELECT observedTime, threadCount FROM FG_JAVA_THREAD WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	briefStatSQL  = `SELECT observedTime, runnableCount, blockedCount, waitingCount, deadlockCount FROM FG_JAVA_THREAD_STATE WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	briefPauseSQL = `SELECT observedTime, collectorName, pauseMaxMs FROM FG_JAVA_GC_PAUSE WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
)

// sampleWindow 是判断用的最近几个点，旧的在前。
type sampleWindow struct {
	heapPct     []float64
	oldUsed     []float64
	metaspace   []float64
	loaded      []float64
	directUsed  []float64
	directMax   float64
	threads     []float64
	runnable    float64
	blocked     float64
	waiting     float64
	hasState    bool
	cpu         float64
	hasCPU      bool
	overhead    float64
	hasOverhead bool
	deadlock    float64
	hasDeadlock bool
	oldPause    []float64
	pauseMax    float64
	hasPause    bool
}

type findingItem struct {
	level string
	text  string
}

// judgeWindow 根据最近几个点给出最多三句结论。材料不够时说明还看不出来。
func judgeWindow(w sampleWindow) (string, []string) {
	if len(w.heapPct) == 0 && len(w.threads) == 0 && len(w.oldUsed) == 0 && !w.hasState && !w.hasCPU && !w.hasPause {
		return findingOK, []string{findingShort}
	}
	var items []findingItem
	add := func(level, text string) {
		if len(items) >= 3 || text == "" {
			return
		}
		items = append(items, findingItem{level: level, text: text})
	}
	oldUp := rising(w.oldUsed, 1.15, 32<<20)
	pauseUp := pauseGrew(w.oldPause)
	switch {
	case oldUp && pauseUp:
		add(findingAlert, "老年代持续上升，Old 停顿变长")
	case oldUp:
		add(findingAlert, "老年代持续上升")
	case pauseUp:
		add(findingAlert, "Old 停顿变长")
	}
	if pct, ok := latest(w.heapPct); ok && shownPercent(pct) >= heapAlertPercent {
		add(findingAlert, "堆使用率已经贴着上限")
	}
	directLatest, hasDirect := latest(w.directUsed)
	directNear := hasDirect && w.directMax > 0 && directLatest/w.directMax >= 0.8
	if directNear {
		add(findingAlert, "直接内存接近上限")
	}
	if w.hasState {
		threads, _ := latest(w.threads)
		if w.blocked >= 8 || (w.blocked >= 5 && threads > 0 && w.blocked/threads >= 0.2) {
			add(findingAlert, "有线程堵在锁上")
		}
	}
	if w.hasCPU && w.cpu >= 60 && w.hasPause && w.pauseMax >= 100 {
		add(findingAlert, "进程 CPU 高，回收也在占时间")
	}
	if w.hasOverhead && w.overhead >= overheadAlertPercent {
		add(findingAlert, "这一拍停顿开销 "+formatPercent(w.overhead))
	}
	if w.hasDeadlock && w.deadlock >= 1 {
		add(findingAlert, "检测到 "+strconv.FormatInt(int64(w.deadlock), 10)+" 个死锁线程")
	}
	if w.hasPause && w.pauseMax >= pauseWatchMs && !pauseUp {
		level := findingWatch
		if w.pauseMax >= pauseAlertMs {
			level = findingAlert
		}
		add(level, "最近出现了较长停顿")
	}
	if rising(w.metaspace, 1.1, 8<<20) && rising(w.loaded, 1.1, 50) {
		add(findingWatch, "元空间和已加载类一起上涨")
	}
	if !directNear && rising(w.directUsed, 1.25, 16<<20) {
		add(findingWatch, "直接内存持续上涨")
	}
	if rising(w.threads, 1.25, 20) {
		add(findingWatch, "线程数持续上升")
	}
	if w.hasCPU && w.cpu >= 70 && w.hasState && w.runnable >= 4 && w.blocked <= 2 && (!w.hasPause || w.pauseMax < 80) {
		add(findingWatch, "进程 CPU 高，线程在跑业务")
	}
	if len(items) == 0 {
		return findingOK, []string{findingQuiet}
	}
	level := findingWatch
	lines := make([]string, 0, len(items))
	for _, item := range items {
		if item.level == findingAlert {
			level = findingAlert
		}
		lines = append(lines, item.text)
	}
	return level, lines
}

func attachFindings(db *dbx.DB, tenantID string, rows []models.JavaRow) {
	for i := range rows {
		level, lines := judgeAgent(db, tenantID, rows[i].AgentID)
		rows[i].FindingLevel = level
		rows[i].Findings = lines
	}
}

func judgeAgent(db *dbx.DB, tenantID, agentID string) (string, []string) {
	if db == nil || agentID == "" {
		return findingOK, []string{findingShort}
	}
	return judgeWindow(loadWindow(db, tenantID, agentID))
}

func loadWindow(db *dbx.DB, tenantID, agentID string) sampleWindow {
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID}
	var w sampleWindow
	for _, row := range queryWide(db, briefHeapSQL, args, 6, 3) {
		if pct := asFloat(row[1]); pct >= 0 {
			w.heapPct = append(w.heapPct, pct)
		}
		w.metaspace = append(w.metaspace, asFloat(row[2]))
	}
	w.heapPct = chronological(w.heapPct, 6)
	w.metaspace = chronological(w.metaspace, 6)
	var oldUsed, directUsed []float64
	for _, row := range queryWide(db, briefPoolSQL, args, 16, 3) {
		switch strings.ToLower(strings.TrimSpace(textOf(row[1]))) {
		case "old":
			oldUsed = append(oldUsed, asFloat(row[2]))
		case "direct":
			directUsed = append(directUsed, asFloat(row[2]))
		}
	}
	w.oldUsed = chronological(oldUsed, 6)
	w.directUsed = chronological(directUsed, 6)
	if rows := queryWide(db, briefMaxSQL, args, 1, 2); len(rows) > 0 {
		w.directMax = asFloat(rows[0][1])
	}
	runtimeRows := queryWide(db, briefRunSQL, args, 6, 4)
	if len(runtimeRows) > 0 {
		if cpu := asFloat(runtimeRows[0][1]); cpu >= 0 {
			w.cpu = cpu
			w.hasCPU = true
		}
		if overhead := asFloat(runtimeRows[0][3]); overhead >= 0 {
			w.overhead = overhead
			w.hasOverhead = true
		}
	}
	for _, row := range runtimeRows {
		w.loaded = append(w.loaded, asFloat(row[2]))
	}
	w.loaded = chronological(w.loaded, 6)
	for _, row := range queryWide(db, briefThrSQL, args, 6, 2) {
		w.threads = append(w.threads, asFloat(row[1]))
	}
	w.threads = chronological(w.threads, 6)
	if rows := queryWide(db, briefStatSQL, args, 1, 5); len(rows) > 0 {
		w.hasState = true
		w.runnable = asFloat(rows[0][1])
		w.blocked = asFloat(rows[0][2])
		w.waiting = asFloat(rows[0][3])
		if dead := asFloat(rows[0][4]); dead >= 0 {
			w.deadlock = dead
			w.hasDeadlock = true
		}
	}
	var ticks []pauseTick
	var current string
	for _, row := range queryWide(db, briefPauseSQL, args, 24, 3) {
		at := timeText(row[0])
		if at == "" {
			continue
		}
		if at != current {
			ticks = append(ticks, pauseTick{})
			current = at
		}
		tick := &ticks[len(ticks)-1]
		value := asFloat(row[2])
		if value > tick.peak {
			tick.peak = value
		}
		if oldPauseCollector(textOf(row[1])) {
			tick.sawOld = true
			if value > tick.old {
				tick.old = value
			}
		}
	}
	if len(ticks) > 6 {
		ticks = ticks[:6]
	}
	for i, j := 0, len(ticks)-1; i < j; i, j = i+1, j-1 {
		ticks[i], ticks[j] = ticks[j], ticks[i]
	}
	for _, tick := range ticks {
		w.hasPause = true
		if tick.sawOld {
			w.oldPause = append(w.oldPause, tick.old)
		}
	}
	if n := len(ticks); n > 0 {
		w.pauseMax = ticks[n-1].peak
		w.hasPause = true
	}
	return w
}

type pauseTick struct {
	peak   float64
	old    float64
	sawOld bool
}

func chronological(newestFirst []float64, keep int) []float64 {
	if len(newestFirst) > keep {
		newestFirst = newestFirst[:keep]
	}
	out := append([]float64(nil), newestFirst...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func latest(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	return values[len(values)-1], true
}

func rising(values []float64, ratio, minDelta float64) bool {
	if len(values) < 4 {
		return false
	}
	window := values[len(values)-4:]
	for i := 1; i < len(window); i++ {
		if window[i]+1 < window[i-1] {
			return false
		}
	}
	delta := window[3] - window[0]
	if delta < minDelta {
		return false
	}
	if window[0] <= 0 {
		return true
	}
	return window[3] >= window[0]*ratio
}

func pauseGrew(values []float64) bool {
	if len(values) < 2 {
		return false
	}
	last := values[len(values)-1]
	earlier := 0.0
	for _, value := range values[:len(values)-1] {
		if value > earlier {
			earlier = value
		}
	}
	return last >= 50 && earlier > 0 && last >= earlier*1.5 && last-earlier >= 20
}

// shownPercent 与页面一位小数对齐。84.95 显示成 85.0，判断也按 85.0 比较。
func shownPercent(value float64) float64 {
	text := strconv.FormatFloat(value, 'f', 1, 64)
	shown, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return value
	}
	return shown
}

// formatPercent 保留三位，和代理写入的开销精度一致，避免 9.960 被显示成 10.0。
func formatPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64) + "%"
}

// oldPauseCollector 只认老年代或 Major 的停顿。周期和年轻代停顿不算。
func oldPauseCollector(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || strings.Contains(n, "cycle") {
		return false
	}
	if strings.Contains(n, "young") || strings.Contains(n, "scavenge") || strings.Contains(n, "parnew") || n == "copy" {
		return false
	}
	if strings.Contains(n, "minor") && strings.Contains(n, "pause") {
		return false
	}
	if strings.Contains(n, "old") || strings.Contains(n, "tenured") || strings.Contains(n, "marksweep") {
		return true
	}
	return strings.Contains(n, "major") && strings.Contains(n, "pause")
}
