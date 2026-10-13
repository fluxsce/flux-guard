package dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/store"
	"fluxguard/server/web/ingest/service"
	"fluxguard/server/web/views/fg0006/models"
	dbx "github.com/fluxsce/dbx/db"
)

func TestJavaListAndSeries(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := service.Open(db)
	now := time.Date(2026, 9, 30, 2, 0, 0, 0, time.UTC)
	later := now.Add(time.Minute)
	first := protocol.Batch{
		TenantID: "default", AgentID: "java-order", BatchID: "batch-java-a", AgentCode: "java", SentAt: now,
		Java: &protocol.Java{AppName: "order", JavaVersion: "21", HostName: "box", IPAddress: "10.0.0.8", StartTime: now},
		Samples: []protocol.Sample{
			{MetricName: "jvm.heap.used.bytes", MetricValue: 100, ObservedAt: now},
			{MetricName: "jvm.heap.max.bytes", MetricValue: 400, ObservedAt: now},
			{MetricName: "jvm.thread.count", MetricValue: 8, ObservedAt: now},
			{MetricName: "jvm.gc.count", MetricValue: 2, Labels: map[string]string{"collector": "G1 Young"}, ObservedAt: now},
			{MetricName: "jvm.gc.time.ms", MetricValue: 10, Labels: map[string]string{"collector": "G1 Young"}, ObservedAt: now},
			{MetricName: "jvm.memory.pool.used.bytes", MetricValue: 80, Labels: map[string]string{"pool": "old"}, ObservedAt: now},
			{MetricName: "jvm.gc.pause.max.ms", MetricValue: 6, Labels: map[string]string{"collector": "G1 Young"}, ObservedAt: now},
		},
	}
	if _, err := svc.Accept(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.BatchID = "batch-java-b"
	second.SentAt = later
	second.Java = nil
	second.Samples = []protocol.Sample{
		{MetricName: "jvm.heap.used.bytes", MetricValue: 200, ObservedAt: later},
		{MetricName: "jvm.heap.max.bytes", MetricValue: 400, ObservedAt: later},
		{MetricName: "jvm.thread.count", MetricValue: 10, ObservedAt: later},
		{MetricName: "jvm.gc.count", MetricValue: 5, Labels: map[string]string{"collector": "G1 Young"}, ObservedAt: later},
		{MetricName: "jvm.gc.time.ms", MetricValue: 40, Labels: map[string]string{"collector": "G1 Young"}, ObservedAt: later},
	}
	if _, err := svc.Accept(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	rows, total, err := List(db, "default", JavaFilter{AppName: "ord"}, dbx.Page{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("total=%d rows=%d", total, len(rows))
	}
	row := rows[0]
	if row.AppName != "order" || row.HeapUsedBytes != 200 || row.HeapUsedPercent != 50 || row.ThreadCount != 10 || row.CollectionCount != 5 {
		t.Fatalf("row %+v", row)
	}
	if row.FindingLevel != findingOK || len(row.Findings) != 1 || row.Findings[0] != findingQuiet {
		t.Fatalf("finding %s %+v", row.FindingLevel, row.Findings)
	}
	detail := Detail(db, "default", "java-order")
	if len(detail.Collectors) != 1 || detail.Collectors[0].CollectorName != "G1 Young" || detail.Collectors[0].CollectionCount != 5 {
		t.Fatalf("detail %+v", detail)
	}
	series := Series(db, "default", "java-order", SeriesSpan{})
	found := map[string]int{}
	for _, curve := range series.Curves {
		found[curve.Group+"/"+curve.Name] = len(curve.Points)
	}
	if found["/堆已用"] != 2 || found["/线程数"] != 2 || found["GC 次数/G1 Young"] != 2 || found["/老年代"] != 1 || found["GC 最长停顿/G1 Young"] != 1 {
		t.Fatalf("curves %+v", found)
	}
	var rate float64
	for _, curve := range series.Curves {
		if curve.Group == "GC 频率" && curve.Name == "G1 Young" && len(curve.Points) == 1 {
			rate = curve.Points[0].Value
		}
	}
	if rate < 0.04 || rate > 0.06 {
		t.Fatalf("gc rate=%v curves=%+v", rate, found)
	}
	if len(series.Restarts) != 0 || detail.Previous != nil {
		t.Fatalf("restart %+v previous %+v", series.Restarts, detail.Previous)
	}
}

func TestJavaRestartKeepsSinceStart(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := service.Open(db)
	boot := time.Date(2026, 9, 30, 2, 0, 0, 0, time.UTC)
	next := boot.Add(time.Minute)
	again := boot.Add(2 * time.Minute)
	post := func(id string, at time.Time, start time.Time, pid string, count float64) {
		t.Helper()
		batch := protocol.Batch{
			TenantID: "default", AgentID: "java-order", BatchID: id, AgentCode: "java", SentAt: at,
			Java: &protocol.Java{AppName: "order", JavaVersion: "21", ProcessID: pid, StartTime: start},
			Samples: []protocol.Sample{
				{MetricName: "jvm.gc.count", MetricValue: count, Labels: map[string]string{"collector": "G1 Young"}, ObservedAt: at},
				{MetricName: "jvm.gc.time.ms", MetricValue: count * 10, Labels: map[string]string{"collector": "G1 Young"}, ObservedAt: at},
			},
		}
		if _, err := svc.Accept(context.Background(), batch); err != nil {
			t.Fatal(err)
		}
	}
	post("batch-run-a", boot, boot, "41", 2)
	post("batch-run-b", next, boot, "41", 5)
	post("batch-run-c", again, again, "99", 1)

	rows, _, err := List(db, "default", JavaFilter{}, dbx.Page{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].CollectionCount != 1 || rows[0].ProcessID != "99" {
		t.Fatalf("current %+v", rows)
	}
	detail := Detail(db, "default", "java-order")
	if len(detail.Collectors) != 1 || detail.Collectors[0].CollectionCount != 1 {
		t.Fatalf("current collectors %+v", detail.Collectors)
	}
	if detail.Previous == nil || detail.Previous.CollectionCount != 5 || detail.Previous.ProcessID != "41" {
		t.Fatalf("previous %+v", detail.Previous)
	}
	if len(detail.Previous.Collectors) != 1 || detail.Previous.Collectors[0].CollectionCount != 5 {
		t.Fatalf("previous collectors %+v", detail.Previous.Collectors)
	}
	series := Series(db, "default", "java-order", SeriesSpan{})
	var rates int
	var counts int
	for _, curve := range series.Curves {
		if curve.Group == "GC 频率" && curve.Name == "G1 Young" {
			rates = len(curve.Points)
		}
		if curve.Group == "GC 次数" && curve.Name == "G1 Young" {
			counts = len(curve.Points)
		}
	}
	if counts != 3 || rates != 1 || len(series.Restarts) != 1 {
		t.Fatalf("counts=%d rates=%d restarts=%+v", counts, rates, series.Restarts)
	}
}

func TestJavaBeatMatchesJudge(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := service.Open(db)
	now := time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)
	later := now.Add(15 * time.Second)
	young := map[string]string{"collector": "G1 Young"}
	first := protocol.Batch{
		TenantID: "default", AgentID: "java-beat", BatchID: "batch-beat-a", AgentCode: "java", SentAt: now,
		Java: &protocol.Java{AppName: "pay", JavaVersion: "21", ProcessID: "7", StartTime: now},
		Samples: []protocol.Sample{
			{MetricName: "jvm.heap.used.bytes", MetricValue: 100, ObservedAt: now},
			{MetricName: "jvm.heap.max.bytes", MetricValue: 400, ObservedAt: now},
			{MetricName: "jvm.thread.count", MetricValue: 8, ObservedAt: now},
			{MetricName: "jvm.gc.pause.overhead.percent", MetricValue: 9.96, ObservedAt: now},
			{MetricName: "jvm.thread.deadlock.count", MetricValue: 0, ObservedAt: now},
			{MetricName: "jvm.gc.pause.count", MetricValue: 2, Labels: young, ObservedAt: now},
			{MetricName: "jvm.gc.pause.max.ms", MetricValue: 40, Labels: map[string]string{"collector": "G1 Young", "cause": "Allocation Failure"}, ObservedAt: now},
			{MetricName: "jvm.gc.pause.time.ms", MetricValue: 50, Labels: young, ObservedAt: now},
			{MetricName: "jvm.gc.pause.p50.ms", MetricValue: 10, Labels: young, ObservedAt: now},
			{MetricName: "jvm.gc.pause.p99.ms", MetricValue: 40, Labels: young, ObservedAt: now},
			{MetricName: "jvm.gc.pause.heap.before.bytes", MetricValue: 1000, Labels: young, ObservedAt: now},
			{MetricName: "jvm.gc.pause.heap.after.bytes", MetricValue: 800, Labels: young, ObservedAt: now},
			{MetricName: "jvm.gc.pause.promoted.bytes", MetricValue: 20, Labels: young, ObservedAt: now},
			{MetricName: "jvm.gc.pause.count", MetricValue: 1, Labels: map[string]string{"collector": "PS Scavenge"}, ObservedAt: now},
			{MetricName: "jvm.gc.pause.max.ms", MetricValue: 15, Labels: map[string]string{"collector": "PS Scavenge"}, ObservedAt: now},
		},
	}
	if _, err := svc.Accept(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	detail := Detail(db, "default", "java-beat")
	if formatPercent(detail.Beat.OverheadPercent) != "9.960%" || detail.Beat.DeadlockCount != 0 {
		t.Fatalf("first beat %+v", detail.Beat)
	}
	for _, line := range detail.Findings {
		if strings.Contains(line, "停顿开销") || strings.Contains(line, "死锁") {
			t.Fatalf("first finding %+v", detail.Findings)
		}
	}
	if len(detail.Beat.Pauses) != 2 {
		t.Fatalf("first pauses %+v", detail.Beat.Pauses)
	}

	second := protocol.Batch{
		TenantID: "default", AgentID: "java-beat", BatchID: "batch-beat-b", AgentCode: "java", SentAt: later,
		Samples: []protocol.Sample{
			{MetricName: "jvm.heap.used.bytes", MetricValue: 200, ObservedAt: later},
			{MetricName: "jvm.heap.max.bytes", MetricValue: 400, ObservedAt: later},
			{MetricName: "jvm.thread.count", MetricValue: 8, ObservedAt: later},
			{MetricName: "jvm.gc.pause.overhead.percent", MetricValue: 12.34, ObservedAt: later},
			{MetricName: "jvm.thread.deadlock.count", MetricValue: 2, Labels: map[string]string{"detail": "pool-1 waits on pool-2"}, ObservedAt: later},
			{MetricName: "jvm.gc.pause.count", MetricValue: 1, Labels: map[string]string{"collector": "G1 Old"}, ObservedAt: later},
			{MetricName: "jvm.gc.pause.max.ms", MetricValue: 600, Labels: map[string]string{"collector": "G1 Old", "cause": "G1 Evacuation Pause"}, ObservedAt: later},
			{MetricName: "jvm.gc.pause.time.ms", MetricValue: 600, Labels: map[string]string{"collector": "G1 Old"}, ObservedAt: later},
			{MetricName: "jvm.gc.pause.p99.ms", MetricValue: 600, Labels: map[string]string{"collector": "G1 Old"}, ObservedAt: later},
			{MetricName: "jvm.gc.cycle.count", MetricValue: 1, Labels: map[string]string{"collector": "ZGC Major Cycles"}, ObservedAt: later},
			{MetricName: "jvm.gc.cycle.time.ms", MetricValue: 80, Labels: map[string]string{"collector": "ZGC Major Cycles"}, ObservedAt: later},
		},
	}
	if _, err := svc.Accept(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	detail = Detail(db, "default", "java-beat")
	if formatPercent(detail.Beat.OverheadPercent) != "12.340%" || detail.Beat.DeadlockCount != 2 || !strings.Contains(detail.Beat.DeadlockText, "pool-1") {
		t.Fatalf("beat %+v", detail.Beat)
	}
	if !hasLine(detail.Findings, "这一拍停顿开销 12.340%") || !hasLine(detail.Findings, "检测到 2 个死锁线程") || !hasLine(detail.Findings, "最近出现了较长停顿") {
		t.Fatalf("findings %+v", detail.Findings)
	}
	byName := map[string]models.JavaPause{}
	for _, row := range detail.Beat.Pauses {
		byName[row.CollectorName] = row
	}
	old := byName["G1 Old"]
	if old.PauseP99Ms != 600 || old.PromotedBytes != -1 || old.PauseCause != "G1 Evacuation Pause" {
		t.Fatalf("old %+v", old)
	}
	cycle := byName["ZGC Major Cycles"]
	if cycle.PauseCount != 0 || cycle.CycleTimeMs != 80 || cycle.PauseP99Ms != -1 || cycle.PauseMaxMs != 0 {
		t.Fatalf("cycle %+v", cycle)
	}
	if _, ok := byName["G1 Young"]; ok {
		t.Fatalf("latest beat kept an older collector %+v", byName)
	}

	series := Series(db, "default", "java-beat", SeriesSpan{})
	found := map[string]int{}
	for _, curve := range series.Curves {
		found[curve.Group+"/"+curve.Name] = len(curve.Points)
		if curve.Group == "GC 最长停顿" && curve.Name == "ZGC Major Cycles" {
			t.Fatalf("cycle plotted as pause %+v", curve.Points)
		}
		if curve.Group == "GC p99" && curve.Name == "PS Scavenge" {
			t.Fatalf("missing p99 plotted as a point")
		}
	}
	if found["/堆使用率"] != 2 || found["GC p99/G1 Young"] != 1 || found["GC p99/G1 Old"] != 1 || found["GC 周期/ZGC Major Cycles"] != 1 || found["GC 开销/停顿开销"] != 2 {
		t.Fatalf("curves %+v", found)
	}
	var dead float64 = -1
	for _, curve := range series.Curves {
		if curve.Name == "死锁线程" && len(curve.Points) > 0 {
			dead = curve.Points[len(curve.Points)-1].Value
		}
	}
	if dead != 2 {
		t.Fatalf("deadlock curve %v", dead)
	}
}

func TestJavaHeapWithoutMaxStaysUnknown(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := service.Open(db)
	now := time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC)
	if _, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "java-nomax", BatchID: "batch-nomax", AgentCode: "java", SentAt: now,
		Java: &protocol.Java{AppName: "nomax", JavaVersion: "21", StartTime: now},
		Samples: []protocol.Sample{
			{MetricName: "jvm.heap.used.bytes", MetricValue: 500, ObservedAt: now},
			{MetricName: "jvm.heap.max.bytes", MetricValue: -1, ObservedAt: now},
			{MetricName: "jvm.thread.count", MetricValue: 4, ObservedAt: now},
		},
	}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := List(db, "default", JavaFilter{}, dbx.Page{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].HeapUsedPercent != -1 || rows[0].HeapMaxBytes != -1 {
		t.Fatalf("row %+v", rows)
	}
	for _, line := range rows[0].Findings {
		if strings.Contains(line, "堆使用率") {
			t.Fatalf("unknown heap alerted %+v", rows[0].Findings)
		}
	}
	series := Series(db, "default", "java-nomax", SeriesSpan{})
	for _, curve := range series.Curves {
		if curve.Name == "堆使用率" || curve.Name == "堆上限" {
			t.Fatalf("plotted %s %+v", curve.Name, curve.Points)
		}
	}
}

func TestJavaHeapPercentMatchesJudge(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := service.Open(db)
	now := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	if _, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "java-pct", BatchID: "batch-pct", AgentCode: "java", SentAt: now,
		Java: &protocol.Java{AppName: "pct", JavaVersion: "21", StartTime: now},
		Samples: []protocol.Sample{
			{MetricName: "jvm.heap.used.bytes", MetricValue: 8496, ObservedAt: now},
			{MetricName: "jvm.heap.max.bytes", MetricValue: 10000, ObservedAt: now},
			{MetricName: "jvm.thread.count", MetricValue: 4, ObservedAt: now},
		},
	}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := List(db, "default", JavaFilter{}, dbx.Page{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].HeapUsedPercent != 85 || !hasLine(rows[0].Findings, "堆使用率已经贴着上限") {
		t.Fatalf("list %+v", rows)
	}
	series := Series(db, "default", "java-pct", SeriesSpan{})
	var plotted float64 = -1
	for _, curve := range series.Curves {
		if curve.Name == "堆使用率" && len(curve.Points) == 1 {
			plotted = curve.Points[0].Value
		}
	}
	if plotted != 85 {
		t.Fatalf("chart percent %v", plotted)
	}
}
