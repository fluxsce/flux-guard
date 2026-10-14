package report_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"fluxguard/agents/goruntime"
	"fluxguard/agents/host"
	"fluxguard/common/report"
	"fluxguard/protocol"
	"fluxguard/server/setting"
	"fluxguard/server/store"
	webinit "fluxguard/server/web/init"
	login "fluxguard/server/web/views/login/controllers"
)

func TestReportAppRuntime(t *testing.T) {
	authDB, authCleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authCleanup)
	if err := setting.Load(context.Background(), authDB); err != nil {
		t.Fatal(err)
	}
	login.Open(authDB, "default")
	srv := httptest.NewServer(webinit.New(authDB))
	defer srv.Close()

	result, err := goruntime.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Go == nil || result.Go.GoVersion == "" || result.Go.GoroutineCount <= 0 {
		t.Fatalf("go snapshot %+v", result.Go)
	}
	err = report.Report(context.Background(), report.Config{
		ServerURL:  srv.URL,
		AgentToken: "agt_dev_local",
	}, srv.Client(), goruntime.Code, result)
	if err != nil {
		t.Fatal(err)
	}
	var sampleCount int
	var agentCode string
	if err := store.Scan(context.Background(), authDB, `
		SELECT sampleCount, agentCode FROM FG_INGEST_BATCH WHERE tenantId = 'default' AND activeFlag = 'Y'`, nil, &sampleCount, &agentCode); err != nil {
		t.Fatal(err)
	}
	if sampleCount != len(result.Samples) || agentCode != goruntime.Code {
		t.Fatalf("sampleCount=%d agentCode=%s", sampleCount, agentCode)
	}
	var routines int
	var version string
	if err := store.Scan(context.Background(), authDB, `
		SELECT goroutineCount, COALESCE(goVersion, '') FROM FG_AGENT_GO WHERE tenantId = 'default' AND activeFlag = 'Y'`, nil, &routines, &version); err != nil {
		t.Fatal(err)
	}
	if routines != result.Go.GoroutineCount || version != result.Go.GoVersion {
		t.Fatalf("stored routines=%d version=%s", routines, version)
	}
}

func TestReportSplitsOversizedBatch(t *testing.T) {
	authDB, authCleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authCleanup)
	if err := setting.Load(context.Background(), authDB); err != nil {
		t.Fatal(err)
	}
	login.Open(authDB, "default")
	srv := httptest.NewServer(webinit.New(authDB))
	defer srv.Close()

	samples := make([]protocol.Sample, 1001)
	for i := range samples {
		samples[i] = protocol.Sample{MetricName: "runtime.gauge", MetricValue: float64(i)}
	}
	err = report.Report(context.Background(), report.Config{
		ServerURL:  srv.URL,
		AgentToken: "agt_dev_local",
	}, srv.Client(), "host", protocol.CollectResult{Samples: samples})
	if err != nil {
		t.Fatal(err)
	}
	var got int
	if err := store.Scan(context.Background(), authDB, `
		SELECT COALESCE(sum(sampleCount), 0) FROM FG_INGEST_BATCH WHERE tenantId = 'default' AND activeFlag = 'Y'`, nil, &got); err != nil {
		t.Fatal(err)
	}
	if got != 1001 {
		t.Fatalf("samples=%d", got)
	}
}

func TestHostCollect(t *testing.T) {
	ctx := context.Background()
	if _, err := host.Collect(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	result, err := host.Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, sample := range result.Samples {
		names[sample.MetricName] = true
	}
	for _, want := range []string{
		"host.cpu.count",
		"host.cpu.usage.percent",
		"host.mem.total.bytes",
		"host.mem.used.percent",
		"host.disk.total.bytes",
		"host.disk.io.read.bytes",
		"host.net.receive.bytes",
		"host.proc.count",
		"host.proc.rss.bytes",
		"host.net.tcp.established",
		"host.fd.allocated",
		"host.uptime.seconds",
	} {
		if !names[want] {
			t.Fatalf("missing %s, samples=%d", want, len(result.Samples))
		}
	}
	if result.Host == nil || result.Host.HostName == "" || result.Host.OSName == "" || result.Host.CPUCount <= 0 || result.Host.MemTotalBytes == 0 || result.Host.MachineID == "" {
		t.Fatalf("host=%+v", result.Host)
	}
}
