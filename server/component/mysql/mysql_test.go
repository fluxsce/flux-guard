package mysql

import "testing"

func TestParseStatus(t *testing.T) {
	sample := ParseStatus(map[string]string{
		"Threads_connected":                "12",
		"Threads_running":                  "2",
		"Questions":                        "100",
		"Slow_queries":                     "1",
		"Uptime":                           "9",
		"Innodb_buffer_pool_reads":         "1",
		"Innodb_buffer_pool_read_requests": "4",
	})
	if sample.ThreadsConnected != 12 || sample.BufferPoolHitRate != 75 || sample.ReplicaLagSec != -1 {
		t.Fatalf("%+v", sample)
	}
}

func TestCellText(t *testing.T) {
	if cellText([]byte("15")) != "15" || atof(cellText([]byte("15"))) != 15 {
		t.Fatal(cellText([]byte("15")))
	}
}
