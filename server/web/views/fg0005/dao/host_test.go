package dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/store"
	"fluxguard/server/web/ingest/service"
	dbx "github.com/fluxsce/dbx/db"
)

func TestListAndDetail(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := service.Open(db)
	_, err = svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-list-1", AgentCode: "host",
		Host: &protocol.Host{
			HostName: "box", OSName: "linux", ArchName: "amd64", ReportName: "机房A", IPAddress: "10.0.0.8",
		},
		Samples: []protocol.Sample{
			{MetricName: "host.cpu.count", MetricValue: 4},
			{MetricName: "host.cpu.usage.percent", MetricValue: 12.5},
			{MetricName: "host.cpu.load1", MetricValue: 0.4},
			{MetricName: "host.mem.used.percent", MetricValue: 40},
			{MetricName: "host.mem.used.bytes", MetricValue: 800},
			{MetricName: "host.mem.total.bytes", MetricValue: 2000},
			{MetricName: "host.disk.used.percent", MetricValue: 70, Labels: map[string]string{"path": "/", "device": "sda1"}},
			{MetricName: "host.disk.used.bytes", MetricValue: 700, Labels: map[string]string{"path": "/", "device": "sda1"}},
			{MetricName: "host.disk.total.bytes", MetricValue: 1000, Labels: map[string]string{"path": "/", "device": "sda1"}},
			{MetricName: "host.disk.used.bytes", MetricValue: 100, Labels: map[string]string{"path": "/data", "device": "sdb1"}},
			{MetricName: "host.disk.total.bytes", MetricValue: 500, Labels: map[string]string{"path": "/data", "device": "sdb1"}},
			{MetricName: "host.disk.used.bytes", MetricValue: 300, Labels: map[string]string{"path": "C:", "device": "C:"}},
			{MetricName: "host.disk.total.bytes", MetricValue: 900, Labels: map[string]string{"path": "C:", "device": "C:"}},
			{MetricName: "host.disk.used.bytes", MetricValue: 10, Labels: map[string]string{"path": "/s/ac1bc307/volume-subpaths/wms/0", "device": "/dev/sda1"}},
			{MetricName: "host.disk.total.bytes", MetricValue: 1000, Labels: map[string]string{"path": "/s/ac1bc307/volume-subpaths/wms/0", "device": "/dev/sda1"}},
			{MetricName: "host.disk.io.read.bytes_per_sec", MetricValue: 2048, Labels: map[string]string{"device": "sda"}},
			{MetricName: "host.disk.io.write.bytes_per_sec", MetricValue: 512, Labels: map[string]string{"device": "sda"}},
			{MetricName: "host.disk.io.util.percent", MetricValue: 8, Labels: map[string]string{"device": "sda"}},
			{MetricName: "host.disk.io.read.bytes_per_sec", MetricValue: 100, Labels: map[string]string{"device": "sdb"}},
			{MetricName: "host.net.receive.bytes_per_sec", MetricValue: 4096, Labels: map[string]string{"device": "eth0"}},
			{MetricName: "host.net.transmit.bytes_per_sec", MetricValue: 1024, Labels: map[string]string{"device": "eth0"}},
			{MetricName: "host.net.receive.bytes_per_sec", MetricValue: 200, Labels: map[string]string{"device": "eth1"}},
			{MetricName: "host.cpu.core.usage.percent", MetricValue: 20, Labels: map[string]string{"cpu": "0"}},
			{MetricName: "host.net.tcp.established", MetricValue: 12},
			{MetricName: "host.net.tcp.time_wait", MetricValue: 3},
			{MetricName: "host.net.tcp.close_wait", MetricValue: 1},
			{MetricName: "host.fd.allocated", MetricValue: 900},
			{MetricName: "host.fd.used.percent", MetricValue: 9},
			{MetricName: "host.proc.count", MetricValue: 40},
			{MetricName: "host.proc.zombie.count", MetricValue: 1},
			{MetricName: "host.proc.rss.bytes", MetricValue: 4096, Labels: map[string]string{"name": "nginx", "pid": "12"}},
			{MetricName: "host.proc.io.read.bytes_per_sec", MetricValue: 1048576, Labels: map[string]string{"name": "java", "pid": "88"}},
			{MetricName: "host.proc.io.write.bytes_per_sec", MetricValue: 4096, Labels: map[string]string{"name": "java", "pid": "88"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, _, err := List(db, "default", HostFilter{}, dbx.Page{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	item := rows[0]
	if item.ReportName != "机房A" || item.CPUUsagePercent != 12.5 || item.MemUsedPercent != 40 || item.DiskUsedPercent != 70 {
		t.Fatalf("row=%+v", item)
	}
	if item.DiskReadBytesPerSec != 2148 || item.NetReceiveBytesPerSec != 4296 {
		t.Fatalf("io=%+v", item)
	}
	if item.TCPEstablished != 12 || item.TCPTimeWait != 3 || item.FDAllocated != 900 || item.ProcCount != 40 || item.ZombieCount != 1 {
		t.Fatalf("runtime=%+v", item)
	}
	detail := Detail(db, "default", item.AgentID)
	if len(detail.Disks) != 3 || detail.Disks[0].MountPoint == "" || len(detail.DiskIO) != 2 || len(detail.Nets) != 2 || len(detail.Cores) != 1 {
		t.Fatalf("detail=%+v", detail)
	}
	if detail.TCP.EstablishedCount != 12 || detail.FD.AllocatedCount != 900 || len(detail.Procs) != 1 || detail.Procs[0].ProcessName != "nginx" {
		t.Fatalf("runtime detail=%+v", detail)
	}
	if len(detail.ProcIO) != 1 || detail.ProcIO[0].ProcessName != "java" || detail.ProcIO[0].ReadBytesPerSec != 1048576 || detail.ProcIO[0].WriteBytesPerSec != 4096 {
		t.Fatalf("proc io=%+v", detail.ProcIO)
	}
	series := Series(db, "default", item.AgentID, SeriesSpan{})
	latest := map[string]float64{}
	for _, curve := range series.Curves {
		if len(curve.Points) == 0 {
			t.Fatalf("empty curve %s", curve.Name)
		}
		latest[curve.Name] = curve.Points[len(curve.Points)-1].Value
	}
	if latest["占用核"] != 0.5 || latest["CPU核数"] != 4 || latest["负载"] != 0.4 || latest["ESTABLISHED"] != 12 || latest["TIME_WAIT"] != 3 || latest["CLOSE_WAIT"] != 1 || latest["LISTEN"] != 0 {
		t.Fatalf("series=%+v", series)
	}
	if latest["内存已用"] != 800 || latest["内存总量"] != 2000 || latest["内存缓存"] != 0 || latest["CPU中断"] != 0 || latest["累计重传"] != 0 {
		t.Fatalf("capacity=%+v", latest)
	}
	if latest["进程数"] != 40 || latest["僵尸"] != 1 || latest["句柄数"] != 900 {
		t.Fatalf("proc=%+v", latest)
	}
	if latest["java 88 读"] != 1048576 || latest["java 88 写"] != 4096 {
		t.Fatalf("proc io series=%+v", latest)
	}
	if _, hasCap := latest["句柄上限"]; hasCap {
		t.Fatal("zero file-max should be omitted")
	}
	if _, idle := latest["交换已用"]; idle {
		t.Fatal("idle swap should be omitted")
	}
	disks := map[string]float64{}
	for _, curve := range series.Curves {
		if curve.Group == "" || len(curve.Points) == 0 {
			continue
		}
		disks[curve.Group+" "+curve.Name] = curve.Points[len(curve.Points)-1].Value
	}
	if disks["磁盘容量 sda1 已用"] != 700 || disks["磁盘容量 sdb1 总量"] != 500 || disks["磁盘容量 C: 已用"] != 300 {
		t.Fatalf("disks=%+v", disks)
	}
	for key := range disks {
		if strings.Contains(key, "volume-subpaths") {
			t.Fatalf("container volume leaked: %s", key)
		}
	}
	if disks["磁盘吞吐 sda 读"] != 2048 || disks["磁盘吞吐 sda 写"] != 512 || disks["磁盘吞吐 sdb 读"] != 100 || disks["网卡 eth0 发送"] != 1024 || disks["网卡 eth1 接收"] != 200 {
		t.Fatalf("devices=%+v", disks)
	}
	if disks["磁盘繁忙 sda 繁忙"] != 8 {
		t.Fatalf("util=%+v", disks)
	}
	if disks["进程CPU nginx 12"] != 0 || disks["进程RSS nginx 12"] != 4096 {
		t.Fatalf("top=%+v", disks)
	}
	outside := Series(db, "default", item.AgentID, SeriesSpan{
		From: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	if len(outside.Curves) != 0 {
		t.Fatalf("outside=%+v", outside)
	}
	inside := Series(db, "default", item.AgentID, SeriesSpan{
		From: time.Now().UTC().Add(-time.Hour),
		To:   time.Now().UTC().Add(time.Minute),
	})
	if len(inside.Curves) == 0 {
		t.Fatal("expected points inside the window")
	}
}
