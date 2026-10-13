package dao

import (
	"context"
	"strings"
	"time"

	"fluxguard/common/hostnic"
	"fluxguard/common/hostvol"
	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0005/models"
	dbx "github.com/fluxsce/dbx/db"
)

// SeriesSpan 是曲线查询的时间窗。起止都为空时仍返回最近一批点。
type SeriesSpan struct {
	From time.Time
	To   time.Time
}

const diskSeriesLimit = 24

const cpuSQL = `SELECT observedTime, usagePercent, cpuCount, userPercent, systemPercent, iowaitPercent, irqPercent, softirqPercent, stealPercent, loadAvg1, loadAvg5, loadAvg15 FROM FG_HOST_CPU WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

const memSQL = `SELECT observedTime, usedBytes, availableBytes, totalBytes, cachedBytes, buffersBytes, swapUsedBytes, swapTotalBytes FROM FG_HOST_MEM WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

const tcpSQL = `SELECT observedTime, establishedCount, listenCount, timeWaitCount, closeWaitCount, synSentCount, synRecvCount, finWaitCount, lastAckCount, closingCount, udpInUseCount, retransmitCount FROM FG_HOST_TCP WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

const fdSQL = `SELECT observedTime, allocatedCount, maximumCount FROM FG_HOST_FD WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

const diskSQL = `SELECT observedTime, mountPoint, deviceName, usedBytes, totalBytes FROM FG_HOST_DISK WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

const diskIOSQL = `SELECT observedTime, deviceName, readBytesPerSec, writeBytesPerSec, readCount, writeCount, readLatencyMs, writeLatencyMs, utilPercent FROM FG_HOST_DISK_IO WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

const procSQL = `SELECT observedTime, procCount, runningCount, sleepingCount, zombieCount, rssTotalBytes FROM FG_HOST_PROC WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

const procTopSQL = `SELECT observedTime, processId, processName, rssBytes, cpuPercent FROM FG_HOST_PROC_TOP WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, cpuPercent DESC, pointId DESC`

const procIOSQL = `SELECT observedTime, processId, processName, readBytesPerSec, writeBytesPerSec FROM FG_HOST_PROC_IO WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' ORDER BY observedTime DESC, pointId DESC`

var nicSQL = `SELECT observedTime, nicName, receiveBytesPerSec, transmitBytesPerSec, receiveErrs, transmitErrs, receiveDrop, transmitDrop FROM FG_HOST_NET WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'` + hostnic.SkipSQL("nicName") + ` ORDER BY observedTime DESC, pointId DESC`

// Series 返回一台机器的曲线点，旧的在前。每张表只查一次。未给时间窗时每种曲线取最近 120 个点。
func Series(db *dbx.DB, tenantID, agentID string, span SeriesSpan) models.HostSeries {
	out := models.HostSeries{Curves: []models.Curve{}}
	if db == nil || agentID == "" {
		return out
	}
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID}
	limit := 120
	clause := spanClause(span, args)
	if clause != "" {
		limit = 4000
	}
	build := &seriesBuild{maxPoints: limit, items: map[string]*models.Curve{}}
	appendHost(db, build, clause, args, limit)
	appendDisk(db, build, clause, args, limit)
	appendDiskIO(db, build, clause, args, limit)
	appendNic(db, build, clause, args, limit)
	appendProc(db, build, clause, args, limit)
	build.dropIdleNames("交换已用", "交换总量")
	build.dropIdleGroups("磁盘繁忙")
	out.Curves = build.curves()
	return out
}

func appendHost(db *dbx.DB, build *seriesBuild, clause string, args dbx.Args, limit int) {
	for _, row := range queryWide(db, applySpan(cpuSQL, clause), args, limit, 12) {
		usage := asFloat(row[1])
		cores := asFloat(row[2])
		build.add("占用核", "cores", "", row[0], usage*cores/100)
		build.add("CPU核数", "cores", "", row[0], cores)
		build.add("CPU用户", "percent", "", row[0], asFloat(row[3]))
		build.add("CPU系统", "percent", "", row[0], asFloat(row[4]))
		build.add("CPU等待", "percent", "", row[0], asFloat(row[5]))
		build.add("CPU中断", "percent", "", row[0], asFloat(row[6]))
		build.add("CPU软中断", "percent", "", row[0], asFloat(row[7]))
		build.add("CPU窃取", "percent", "", row[0], asFloat(row[8]))
		build.add("负载", "load", "", row[0], asFloat(row[9]))
		build.add("负载5", "load", "", row[0], asFloat(row[10]))
		build.add("负载15", "load", "", row[0], asFloat(row[11]))
	}
	for _, row := range queryWide(db, applySpan(memSQL, clause), args, limit, 8) {
		build.add("内存已用", "bytes", "", row[0], asFloat(row[1]))
		build.add("内存可用", "bytes", "", row[0], asFloat(row[2]))
		build.add("内存总量", "bytes", "", row[0], asFloat(row[3]))
		build.add("内存缓存", "bytes", "", row[0], asFloat(row[4]))
		build.add("内存缓冲", "bytes", "", row[0], asFloat(row[5]))
		build.add("交换已用", "bytes", "", row[0], asFloat(row[6]))
		build.add("交换总量", "bytes", "", row[0], asFloat(row[7]))
	}
	tcpNames := []string{"ESTABLISHED", "LISTEN", "TIME_WAIT", "CLOSE_WAIT", "SYN_SENT", "SYN_RECV", "FIN_WAIT", "LAST_ACK", "CLOSING", "UDP"}
	for _, row := range queryWide(db, applySpan(tcpSQL, clause), args, limit, 12) {
		for i, name := range tcpNames {
			build.add(name, "count", "", row[0], asFloat(row[i+1]))
		}
		build.add("累计重传", "count", "", row[0], asFloat(row[11]))
	}
	for _, row := range queryWide(db, applySpan(fdSQL, clause), args, limit, 3) {
		build.add("句柄数", "count", "", row[0], asFloat(row[1]))
		if maximum := asFloat(row[2]); maximum > 0 {
			// 上限与已用数差几个数量级。同轴时已用线贴在 0 上，看起来像没有句柄。
			build.add("句柄上限", "cap", "", row[0], maximum)
		}
	}
}

func appendDisk(db *dbx.DB, build *seriesBuild, clause string, args dbx.Args, limit int) {
	for _, row := range queryWide(db, applySpan(diskSQL, clause), args, limit*diskSeriesLimit, 5) {
		label := diskLabel(textOf(row[1]), textOf(row[2]))
		if label == "" || !build.allow("磁盘容量", label) {
			continue
		}
		build.add(label+" 已用", "bytes", "磁盘容量", row[0], asFloat(row[3]))
		build.add(label+" 总量", "bytes", "磁盘容量", row[0], asFloat(row[4]))
	}
}

func appendDiskIO(db *dbx.DB, build *seriesBuild, clause string, args dbx.Args, limit int) {
	newer := map[string]counterSample{}
	for _, row := range queryWide(db, applySpan(diskIOSQL, clause), args, limit*diskSeriesLimit, 9) {
		name := strings.TrimSpace(textOf(row[1]))
		if name == "" || hostvol.Partition(name) || !build.allow("磁盘吞吐", name) {
			continue
		}
		build.add(name+" 读", "rate", "磁盘吞吐", row[0], asFloat(row[2]))
		build.add(name+" 写", "rate", "磁盘吞吐", row[0], asFloat(row[3]))
		build.add(name+" 读延迟", "ms", "磁盘延迟", row[0], asFloat(row[6]))
		build.add(name+" 写延迟", "ms", "磁盘延迟", row[0], asFloat(row[7]))
		build.add(name+" 繁忙", "percent", "磁盘繁忙", row[0], asFloat(row[8]))
		at, ok := store.TimeValue(row[0])
		if !ok {
			continue
		}
		current := counterSample{at: at, text: row[0], values: []float64{asFloat(row[4]), asFloat(row[5])}}
		if prev, seen := newer[name]; seen {
			build.addRate(name+" 读", "iops", "磁盘 IOPS", prev, current, 0)
			build.addRate(name+" 写", "iops", "磁盘 IOPS", prev, current, 1)
		}
		newer[name] = current
	}
}

func appendNic(db *dbx.DB, build *seriesBuild, clause string, args dbx.Args, limit int) {
	newer := map[string]counterSample{}
	for _, row := range queryWide(db, applySpan(nicSQL, clause), args, limit*diskSeriesLimit, 8) {
		name := strings.TrimSpace(textOf(row[1]))
		if name == "" || !build.allow("网卡", name) {
			continue
		}
		build.add(name+" 接收", "rate", "网卡", row[0], asFloat(row[2]))
		build.add(name+" 发送", "rate", "网卡", row[0], asFloat(row[3]))
		at, ok := store.TimeValue(row[0])
		if !ok {
			continue
		}
		current := counterSample{at: at, text: row[0], values: []float64{asFloat(row[4]), asFloat(row[5]), asFloat(row[6]), asFloat(row[7])}}
		if prev, seen := newer[name]; seen {
			build.addRate(name+" 接收错误", "persec", "网卡异常", prev, current, 0)
			build.addRate(name+" 发送错误", "persec", "网卡异常", prev, current, 1)
			build.addRate(name+" 接收丢弃", "persec", "网卡异常", prev, current, 2)
			build.addRate(name+" 发送丢弃", "persec", "网卡异常", prev, current, 3)
		}
		newer[name] = current
	}
}

func appendProc(db *dbx.DB, build *seriesBuild, clause string, args dbx.Args, limit int) {
	for _, row := range queryWide(db, applySpan(procSQL, clause), args, limit, 6) {
		build.add("进程数", "count", "", row[0], asFloat(row[1]))
		build.add("运行", "count", "", row[0], asFloat(row[2]))
		build.add("睡眠", "count", "", row[0], asFloat(row[3]))
		build.add("僵尸", "count", "", row[0], asFloat(row[4]))
		build.add("RSS合计", "bytes", "", row[0], asFloat(row[5]))
	}
	for _, row := range queryWide(db, applySpan(procTopSQL, clause), args, limit*diskSeriesLimit, 5) {
		pid := strings.TrimSpace(textOf(row[1]))
		name := strings.TrimSpace(textOf(row[2]))
		if pid == "" {
			continue
		}
		if name == "" {
			name = pid
		}
		label := name + " " + pid
		if !build.allow("进程CPU", label) {
			continue
		}
		build.add(label, "percent", "进程CPU", row[0], asFloat(row[4]))
		build.add(label, "bytes", "进程RSS", row[0], asFloat(row[3]))
	}
	for _, row := range queryWide(db, applySpan(procIOSQL, clause), args, limit*diskSeriesLimit, 5) {
		pid := strings.TrimSpace(textOf(row[1]))
		name := strings.TrimSpace(textOf(row[2]))
		if pid == "" {
			continue
		}
		if name == "" {
			name = pid
		}
		label := name + " " + pid
		if omitProcIO(pid, name) || !build.allow("进程IO", label) {
			continue
		}
		build.add(label+" 读", "rate", "进程IO", row[0], asFloat(row[3]))
		build.add(label+" 写", "rate", "进程IO", row[0], asFloat(row[4]))
	}
}

// omitProcIO 不把 1 号进程和 systemd 的读写给用户看。
// Linux 收尸时把已退出子进程的历史字节加进它们，速率不是它们自己这一拍的读写。
func omitProcIO(pid, name string) bool {
	if strings.TrimSpace(pid) == "1" {
		return true
	}
	return strings.TrimSpace(name) == "systemd"
}

type seriesBuild struct {
	maxPoints int
	order     []string
	items     map[string]*models.Curve
	kept      map[string]map[string]struct{}
}

func (b *seriesBuild) allow(bucket, key string) bool {
	if b.kept == nil {
		b.kept = map[string]map[string]struct{}{}
	}
	seen := b.kept[bucket]
	if seen == nil {
		seen = map[string]struct{}{}
		b.kept[bucket] = seen
	}
	if _, ok := seen[key]; ok {
		return true
	}
	if len(seen) >= diskSeriesLimit {
		return false
	}
	seen[key] = struct{}{}
	return true
}

type counterSample struct {
	at     time.Time
	text   any
	values []float64
}

func (b *seriesBuild) addRate(name, unit, group string, newer, older counterSample, index int) {
	if index >= len(newer.values) || index >= len(older.values) {
		return
	}
	dt := newer.at.Sub(older.at).Seconds()
	if dt <= 0 || dt > 900 {
		return
	}
	delta := newer.values[index] - older.values[index]
	if delta < 0 {
		delta = 0
	}
	b.add(name, unit, group, newer.text, delta/dt)
}

func (b *seriesBuild) dropIdleNames(names ...string) {
	want := map[string]struct{}{}
	for _, name := range names {
		want[name] = struct{}{}
	}
	var keys []string
	active := false
	for _, key := range b.order {
		curve := b.items[key]
		if curve == nil || curve.Group != "" {
			continue
		}
		if _, ok := want[curve.Name]; !ok {
			continue
		}
		keys = append(keys, key)
		for _, point := range curve.Points {
			if point.Value > 0 {
				active = true
			}
		}
	}
	if active || len(keys) == 0 {
		return
	}
	drop := map[string]struct{}{}
	for _, key := range keys {
		delete(b.items, key)
		drop[key] = struct{}{}
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

func (b *seriesBuild) dropIdleGroups(groups ...string) {
	want := map[string]struct{}{}
	for _, group := range groups {
		want[group] = struct{}{}
	}
	active := map[string]bool{}
	var keys []string
	for _, key := range b.order {
		curve := b.items[key]
		if curve == nil || curve.Group == "" {
			continue
		}
		if _, ok := want[curve.Group]; !ok {
			continue
		}
		keys = append(keys, key)
		for _, point := range curve.Points {
			if point.Value > 0 {
				active[curve.Group] = true
			}
		}
	}
	if len(keys) == 0 {
		return
	}
	drop := map[string]struct{}{}
	for _, key := range keys {
		curve := b.items[key]
		if curve == nil || active[curve.Group] {
			continue
		}
		delete(b.items, key)
		drop[key] = struct{}{}
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

func (b *seriesBuild) add(name, unit, group string, at any, value float64) {
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

func (b *seriesBuild) curves() []models.Curve {
	out := make([]models.Curve, 0, len(b.order))
	for _, key := range b.order {
		curve := b.items[key]
		reversePoints(curve.Points)
		if len(curve.Points) == 0 {
			continue
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

func diskLabel(mount, device string) string {
	if hostvol.Container(mount) {
		return ""
	}
	return hostvol.BlockName(device, mount)
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

func reversePoints(points []models.CurvePoint) []models.CurvePoint {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	return points
}
