package mysql

import (
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	sample := ParseStatus(map[string]string{
		"Threads_connected":                "12",
		"Threads_running":                  "2",
		"Questions":                        "100",
		"Slow_queries":                     "1",
		"Uptime":                           "9",
		"Com_select":                       "40",
		"Innodb_buffer_pool_reads":         "1",
		"Innodb_buffer_pool_read_requests": "4",
		"Innodb_buffer_pool_pages_total":   "100",
		"Innodb_buffer_pool_pages_dirty":   "70",
	}, map[string]string{
		"max_connections":         "100",
		"innodb_buffer_pool_size": "1024",
	})
	if sample.ThreadsConnected != 12 || sample.BufferPoolHitRate != -1 || sample.ReplicaLagSec != -1 {
		t.Fatalf("instant %+v", sample)
	}
	if sample.ConnUsagePercent != 12 || sample.BufferPoolDirtyPercent != 70 || sample.PoolBytes != 1024 {
		t.Fatalf("capacity %+v", sample)
	}
	if sample.Counters.Questions != 100 || sample.Counters.Select != 40 || sample.Counters.PoolReadReqs != 4 {
		t.Fatalf("counters %+v", sample.Counters)
	}
}

func TestApplyRates(t *testing.T) {
	now := time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)
	sample := ParseStatus(map[string]string{
		"Questions":                        "200",
		"Slow_queries":                     "1",
		"Uptime":                           "19",
		"Com_select":                       "50",
		"Com_insert":                       "10",
		"Bytes_received":                   "5120",
		"Innodb_buffer_pool_reads":         "2",
		"Innodb_buffer_pool_read_requests": "8",
		"Created_tmp_tables":               "14",
		"Created_tmp_disk_tables":          "3",
		"Aborted_connects":                 "4",
		"Innodb_row_lock_waits":            "2",
	}, nil)
	prev := &Prior{
		At: now.Add(-10 * time.Second),
		Counters: Counters{
			Questions: 100, Slow: 1, Select: 40, Insert: 0, BytesIn: 1024,
			PoolReads: 1, PoolReadReqs: 4, TmpTables: 10, TmpDisk: 1,
			Aborted: 0, LockWaits: 0, Uptime: 9,
		},
	}
	ApplyRates(&sample, prev, now)
	if sample.QuestionsPerSec != 10 || sample.SlowPerSec != 0 || sample.SelectPerSec != 1 || sample.InsertPerSec != 1 {
		t.Fatalf("rates %+v", sample)
	}
	if sample.BufferPoolHitRate != 75 || sample.TmpDiskPercent != 50 {
		t.Fatalf("ratios hit=%v tmp=%v", sample.BufferPoolHitRate, sample.TmpDiskPercent)
	}
	if sample.NetInKbps != 0.4 || sample.AbortedPerSec != 0.4 || sample.LockWaitsPerSec != 0.2 {
		t.Fatalf("traffic %+v", sample)
	}

	restart := ParseStatus(map[string]string{"Questions": "5", "Uptime": "1"}, nil)
	ApplyRates(&restart, prev, now)
	if restart.QuestionsPerSec != -1 || restart.BufferPoolHitRate != -1 {
		t.Fatalf("restart should stay unknown: %+v", restart)
	}
}

func TestThreadFlag(t *testing.T) {
	if ThreadFlag("Yes") != 1 || ThreadFlag("No") != 0 || ThreadFlag("Connecting") != -1 || ThreadFlag("") != -1 {
		t.Fatal(ThreadFlag("Yes"), ThreadFlag("No"), ThreadFlag("Connecting"))
	}
}

func TestCellText(t *testing.T) {
	if cellText([]byte("15")) != "15" || atof(cellText([]byte("15"))) != 15 {
		t.Fatal(cellText([]byte("15")))
	}
}
