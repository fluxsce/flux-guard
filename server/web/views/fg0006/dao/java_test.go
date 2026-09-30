package dao

import (
	"context"
	"testing"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/store"
	"fluxguard/server/web/ingest/service"
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
}
