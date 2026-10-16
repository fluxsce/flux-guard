package zk

import "testing"

func TestParseSrvr(t *testing.T) {
	sample := ParseSrvr("Zookeeper version: 3.8.4-abc, built on 2022-02-25\nLatency min/avg/max: 0/4/9\nReceived: 26\nSent: 25\nConnections: 1\nOutstanding: 2\nZxid: 0x100\nMode: standalone\nNode count: 5\n")
	if sample.Version != "3.8.4-abc" || sample.Role != "standalone" || sample.AvgLatency != 4 || sample.MinLatency != 0 || sample.MaxLatency != 9 {
		t.Fatalf("%+v", sample)
	}
	if sample.ZnodeCount != 5 || sample.OutstandingRequests != 2 || sample.Connections != 1 || sample.Received != 26 || sample.Zxid != "0x100" {
		t.Fatalf("%+v", sample)
	}
}

func TestParseMntr(t *testing.T) {
	sample := ParseMntr("zk_version\t3.8.0\nzk_server_state\tfollower\nzk_avg_latency\t4\nzk_outstanding_requests\t1\nzk_znode_count\t20\nzk_followers\t2\nzk_watch_count\t7\n")
	if sample.Version != "3.8.0" || sample.Role != "follower" || sample.ZnodeCount != 20 || sample.AvgLatency != 4 || sample.WatchCount != 7 {
		t.Fatalf("%+v", sample)
	}
}
