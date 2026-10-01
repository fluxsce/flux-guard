package zk

import "testing"

func TestParseMntr(t *testing.T) {
	sample := ParseMntr("zk_version\t3.8.0\nzk_server_state\tfollower\nzk_avg_latency\t4\nzk_outstanding_requests\t1\nzk_znode_count\t20\nzk_followers\t2\nzk_watch_count\t7\n")
	if sample.Version != "3.8.0" || sample.Role != "follower" || sample.ZnodeCount != 20 || sample.AvgLatency != 4 || sample.WatchCount != 7 {
		t.Fatalf("%+v", sample)
	}
}
