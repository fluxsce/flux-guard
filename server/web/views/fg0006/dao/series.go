package dao

import (
	"context"
	"strings"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0006/models"
	dbx "github.com/fluxsce/dbx/db"
)

// SeriesSpan 是曲线查询的时间窗。起止都为空时仍返回最近一批点。
type SeriesSpan struct {
	From time.Time
	To   time.Time
}

const (
	heapSQL  = `SELECT observedTime, heapUsedBytes, heapCommittedBytes, heapMaxBytes, nonHeapUsedBytes, nonHeapCommittedBytes, metaspaceUsedBytes, heapUsedPercent FROM FG_JAVA_HEAP WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	gcSQL    = `SELECT observedTime, collectorName, collectionCount, collectionTimeMs FROM FG_JAVA_GC WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	thrSQL   = `SELECT observedTime, threadCount, daemonCount, peakCount FROM FG_JAVA_THREAD WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	poolSQL  = `SELECT observedTime, poolName, usedBytes FROM FG_JAVA_POOL WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	runSQL   = `SELECT observedTime, processCpuPercent, loadedClassCount, unloadedClassCount, pauseOverheadPercent FROM FG_JAVA_RUNTIME WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	pauseSQL = `SELECT observedTime, collectorName, pauseCount, pauseMaxMs, pauseTotalMs, pauseP99Ms, cycleTimeMs FROM FG_JAVA_GC_PAUSE WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	stateSQL = `SELECT observedTime, runnableCount, blockedCount, waitingCount, deadlockCount FROM FG_JAVA_THREAD_STATE WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
	maxSQL   = `SELECT observedTime, poolName, maxBytes FROM FG_JAVA_POOL_MAX WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`
)

// Series 返回一个 Java 进程的曲线点，旧的在前。未给时间窗时每种曲线取最近 120 个点。
func Series(db *dbx.DB, tenantID, agentID string, span SeriesSpan) models.JavaSeries {
	out := models.JavaSeries{Curves: []models.Curve{}, Restarts: []models.JavaRestart{}}
	if db == nil || agentID == "" {
		return out
	}
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID}
	limit := 120
	clause := spanClause(span, args)
	if clause != "" {
		limit = 4000
	}
	build := &curveBuild{maxPoints: limit, items: map[string]*models.Curve{}}
	for _, row := range queryWide(db, applySpan(heapSQL, clause), args, limit, 8) {
		build.add("堆已用", "bytes", "", row[0], asFloat(row[1]))
		build.add("堆已提交", "bytes", "", row[0], asFloat(row[2]))
		if max := asFloat(row[3]); max > 0 {
			build.add("堆上限", "bytes", "", row[0], max)
		}
		if pct := asFloat(row[7]); pct >= 0 {
			build.add("堆使用率", "percent", "", row[0], shownPercent(pct))
		}
		build.add("非堆已用", "bytes", "", row[0], asFloat(row[4]))
		build.add("非堆已提交", "bytes", "", row[0], asFloat(row[5]))
		build.add("元空间已用", "bytes", "", row[0], asFloat(row[6]))
	}
	for _, row := range queryWide(db, applySpan(thrSQL, clause), args, limit, 4) {
		build.add("线程数", "count", "", row[0], asFloat(row[1]))
		build.add("守护线程", "count", "", row[0], asFloat(row[2]))
		build.add("峰值线程", "count", "", row[0], asFloat(row[3]))
	}
	for _, row := range queryWide(db, applySpan(stateSQL, clause), args, limit, 5) {
		build.add("运行线程", "count", "", row[0], asFloat(row[1]))
		build.add("阻塞线程", "count", "", row[0], asFloat(row[2]))
		build.add("等待线程", "count", "", row[0], asFloat(row[3]))
		if dead := asFloat(row[4]); dead >= 0 {
			build.add("死锁线程", "count", "", row[0], dead)
		}
	}
	breaks := loadRestartTimes(db, tenantID, agentID, span)
	var inferred []models.JavaRestart
	newer := map[string]counterSample{}
	for _, row := range queryWide(db, applySpan(gcSQL, clause), args, limit*8, 4) {
		name := strings.TrimSpace(textOf(row[1]))
		if name == "" {
			continue
		}
		build.add(name, "count", "GC 次数", row[0], asFloat(row[2]))
		build.add(name, "ms", "GC 累计时间", row[0], asFloat(row[3]))
		at, ok := store.TimeValue(row[0])
		if !ok {
			continue
		}
		current := counterSample{at: at, text: row[0], values: []float64{asFloat(row[2]), asFloat(row[3])}}
		if prev, seen := newer[name]; seen {
			// 查询按时间倒序，prev 是较新的一拍。次数变小就是这一代 JVM 重新从 0 计。
			reset := len(prev.values) > 0 && len(current.values) > 0 && prev.values[0] < current.values[0]
			crossed := crossesRestart(current.at, prev.at, breaks)
			if reset {
				inferred = append(inferred, models.JavaRestart{ObservedTime: timeText(prev.text)})
			}
			if !reset && !crossed {
				build.addRate(name, "persec", "GC 频率", prev, current, 0)
			}
		}
		newer[name] = current
	}
	for _, row := range queryWide(db, applySpan(poolSQL, clause), args, limit*8, 3) {
		title := poolTitle(textOf(row[1]))
		if title == "" {
			continue
		}
		build.add(title, "bytes", "", row[0], asFloat(row[2]))
	}
	for _, row := range queryWide(db, applySpan(maxSQL, clause), args, limit, 3) {
		if strings.EqualFold(strings.TrimSpace(textOf(row[1])), "direct") {
			build.add("直接内存上限", "bytes", "", row[0], asFloat(row[2]))
		}
	}
	for _, row := range queryWide(db, applySpan(runSQL, clause), args, limit, 5) {
		if cpu := asFloat(row[1]); cpu >= 0 {
			build.add("进程 CPU", "percent", "", row[0], cpu)
		}
		build.add("已加载类", "count", "", row[0], asFloat(row[2]))
		build.add("已卸载类", "count", "", row[0], asFloat(row[3]))
		if overhead := asFloat(row[4]); overhead >= 0 {
			build.add("停顿开销", "percent", "GC 开销", row[0], overhead)
		}
	}
	for _, row := range queryWide(db, applySpan(pauseSQL, clause), args, limit*8, 7) {
		name := strings.TrimSpace(textOf(row[1]))
		if name == "" {
			continue
		}
		count := asFloat(row[2])
		if count > 0 {
			build.add(name, "count", "GC 停顿次数", row[0], count)
		}
		if count > 0 || asFloat(row[3]) > 0 {
			build.add(name, "ms", "GC 最长停顿", row[0], asFloat(row[3]))
		}
		if count > 0 || asFloat(row[4]) > 0 {
			build.add(name, "ms", "GC 区间耗时", row[0], asFloat(row[4]))
		}
		if count > 0 && asFloat(row[5]) >= 0 {
			build.add(name, "ms", "GC p99", row[0], asFloat(row[5]))
		}
		if cycle := asFloat(row[6]); cycle > 0 {
			build.add(name, "ms", "GC 周期", row[0], cycle)
		}
	}
	build.dropIdle("堆上限", "非堆已提交", "元空间已用", "峰值线程", "映射内存", "代码缓存", "压缩类空间", "已卸载类", "阻塞线程", "等待线程", "死锁线程")
	out.Curves = build.curves()
	out.Restarts = mergeRestarts(db, tenantID, agentID, span, inferred)
	return out
}

type curveBuild struct {
	maxPoints int
	order     []string
	items     map[string]*models.Curve
}

type counterSample struct {
	at     time.Time
	text   any
	values []float64
}

func (b *curveBuild) add(name, unit, group string, at any, value float64) {
	observed := timeText(at)
	if observed == "" || name == "" {
		return
	}
	key := group + "\n" + name
	curve := b.items[key]
	if curve == nil {
		curve = &models.Curve{Name: name, Unit: unit, Group: group}
		b.items[key] = curve
		b.order = append(b.order, key)
	}
	if len(curve.Points) >= b.maxPoints {
		return
	}
	curve.Points = append(curve.Points, models.CurvePoint{ObservedTime: observed, Value: value})
}

func (b *curveBuild) addRate(name, unit, group string, newer, older counterSample, index int) {
	if index >= len(newer.values) || index >= len(older.values) {
		return
	}
	dt := newer.at.Sub(older.at).Seconds()
	if dt <= 0 || dt > 900 {
		return
	}
	delta := newer.values[index] - older.values[index]
	if delta < 0 {
		return
	}
	b.add(name, unit, group, newer.text, delta/dt)
}

func (b *curveBuild) dropIdle(names ...string) {
	want := map[string]struct{}{}
	for _, name := range names {
		want[name] = struct{}{}
	}
	drop := map[string]struct{}{}
	for _, key := range b.order {
		curve := b.items[key]
		if curve == nil || curve.Group != "" {
			continue
		}
		if _, ok := want[curve.Name]; !ok {
			continue
		}
		idle := true
		for _, point := range curve.Points {
			if point.Value > 0 {
				idle = false
				break
			}
		}
		if idle {
			delete(b.items, key)
			drop[key] = struct{}{}
		}
	}
	if len(drop) == 0 {
		return
	}
	next := make([]string, 0, len(b.order))
	for _, key := range b.order {
		if _, ok := drop[key]; ok {
			continue
		}
		next = append(next, key)
	}
	b.order = next
}

func (b *curveBuild) curves() []models.Curve {
	out := make([]models.Curve, 0, len(b.order))
	for _, key := range b.order {
		curve := b.items[key]
		if curve == nil || len(curve.Points) == 0 {
			continue
		}
		for i, j := 0, len(curve.Points)-1; i < j; i, j = i+1, j-1 {
			curve.Points[i], curve.Points[j] = curve.Points[j], curve.Points[i]
		}
		out = append(out, *curve)
	}
	return out
}

func queryWide(db *dbx.DB, query string, args dbx.Args, limit, width int) [][]any {
	paged, bound, err := store.PageQuery(db, query, args, limit)
	if err != nil {
		return nil
	}
	rows, err := db.Query(context.Background(), paged, bound)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		dest := make([]any, width)
		ptrs := make([]any, width)
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return out
		}
		out = append(out, dest)
	}
	return out
}

func spanClause(span SeriesSpan, args dbx.Args) string {
	clause := ""
	if !span.From.IsZero() {
		args["fromTime"] = store.FormatTime(span.From)
		clause += " AND observedTime >= @fromTime"
	}
	if !span.To.IsZero() {
		args["toTime"] = store.FormatTime(span.To)
		clause += " AND observedTime <= @toTime"
	}
	return clause
}

func applySpan(query, clause string) string {
	if clause == "" {
		return query
	}
	if i := strings.Index(query, " ORDER BY "); i >= 0 {
		return query[:i] + clause + query[i:]
	}
	return query + clause
}

func poolTitle(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "eden":
		return "年轻代"
	case "survivor":
		return "幸存区"
	case "old":
		return "老年代"
	case "code":
		return "代码缓存"
	case "direct":
		return "直接内存"
	case "mapped":
		return "映射内存"
	case "classspace":
		return "压缩类空间"
	default:
		return ""
	}
}

func textOf(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case []byte:
		return string(n)
	default:
		return ""
	}
}
