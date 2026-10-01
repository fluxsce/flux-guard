package kafka

import "testing"

func TestParseMetadata(t *testing.T) {
	body := appendInt32(nil, 2)
	body = append(body, broker(1, "a", 9092)...)
	body = append(body, broker(2, "b", 9092)...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt16(nil, 0)...)
	body = append(body, kafkaString("orders")...)
	body = append(body, []byte{0}...)
	body = append(body, appendInt32(nil, 2)...)
	body = append(body, partition(0, 0, 1, []int32{1, 2}, []int32{1})...)
	body = append(body, partition(0, 1, -1, []int32{1}, []int32{1})...)
	sample, err := ParseMetadata(body, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sample.BrokerCount != 2 || sample.OfflinePartitions != 1 || sample.UnderReplicated != 1 || sample.PreferredReplicas != 0 {
		t.Fatalf("%+v", sample)
	}
	if sample.ControllerID != 1 || sample.TopicCount != 1 {
		t.Fatalf("%+v", sample)
	}
	if len(sample.Topics) != 1 || sample.Topics[0].Name != "orders" || sample.Topics[0].Offline != 1 || sample.Topics[0].Under != 1 {
		t.Fatalf("%+v", sample.Topics)
	}
}

func TestParseGroups(t *testing.T) {
	groups := appendInt16(nil, 0)
	groups = append(groups, appendInt32(nil, 1)...)
	groups = append(groups, kafkaString("billing")...)
	groups = append(groups, kafkaString("consumer")...)
	names, err := ParseListGroups(groups)
	if err != nil || len(names) != 1 || names[0] != "billing" {
		t.Fatalf("%v %v", names, err)
	}
	coord := appendInt16(nil, 0)
	coord = append(coord, appendInt32(nil, 1)...)
	coord = append(coord, kafkaString("10.0.0.8")...)
	coord = append(coord, appendInt32(nil, 9092)...)
	host, port, err := ParseCoordinator(coord)
	if err != nil || host != "10.0.0.8" || port != 9092 {
		t.Fatalf("%s %d %v", host, port, err)
	}
	fetched := appendInt32(nil, 1)
	fetched = append(fetched, kafkaString("orders")...)
	fetched = append(fetched, appendInt32(nil, 1)...)
	fetched = append(fetched, appendInt32(nil, 0)...)
	fetched = append(fetched, appendInt64(nil, 10)...)
	fetched = append(fetched, appendInt16(nil, -1)...)
	fetched = append(fetched, appendInt16(nil, 0)...)
	commits, err := ParseOffsetFetch(fetched)
	if err != nil || commits["orders"][0] != 10 {
		t.Fatalf("%v %v", commits, err)
	}
	ends := appendInt32(nil, 1)
	ends = append(ends, kafkaString("orders")...)
	ends = append(ends, appendInt32(nil, 1)...)
	ends = append(ends, appendInt32(nil, 0)...)
	ends = append(ends, appendInt16(nil, 0)...)
	ends = append(ends, appendInt64(nil, 1)...)
	ends = append(ends, appendInt64(nil, 25)...)
	latest, err := ParseListOffsets(ends)
	if err != nil || latest["orders"][0] != 25 {
		t.Fatalf("%v %v", latest, err)
	}
	total, maxPart, topic, part, nTopic, ok := summarizeLag(commits, latest)
	if !ok || total != 15 || maxPart != 15 || topic != "orders" || part != 0 || nTopic != 1 {
		t.Fatalf("%d %d %s %v", total, maxPart, topic, ok)
	}
}

func TestMetadataHealth(t *testing.T) {
	body := appendInt32(nil, 0)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, broker(1, "a", 9092)...)
	body = append(body, appendInt16(nil, -1)...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt16(nil, 0)...)
	body = append(body, kafkaString("orders")...)
	body = append(body, []byte{0}...)
	body = append(body, appendInt32(nil, 2)...)
	body = append(body, partition(0, 0, 2, []int32{1, 2}, []int32{1, 2})...)
	body = append(body, appendInt32(nil, 0)...)
	body = append(body, partition(0, 1, 1, []int32{1}, []int32{1})...)
	body = append(body, appendInt32(nil, 0)...)
	sample, err := ParseMetadata(body, 7)
	if err != nil {
		t.Fatal(err)
	}
	if sample.PreferredReplicas != 1 || sample.Topics[0].Skew != 1 || sample.OfflinePartitions != 0 {
		t.Fatalf("%+v %+v", sample, sample.Topics)
	}
	sample.applyMinISR(2, nil)
	if sample.UnderMinIsr != 1 || sample.Topics[0].BelowMin != 1 {
		t.Fatalf("min isr %+v", sample)
	}
}

func TestParseApiAndGroups(t *testing.T) {
	body := appendInt16(nil, 0)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt16(nil, 3)...)
	body = append(body, appendInt16(nil, 1)...)
	body = append(body, appendInt16(nil, 7)...)
	apis, err := ParseApiVersions(body)
	if err != nil {
		t.Fatal(err)
	}
	ver, ok := apis.use(3, 7)
	if !ok || ver != 7 {
		t.Fatalf("%d %v", ver, ok)
	}
	described := appendInt32(nil, 1)
	described = append(described, appendInt16(nil, 0)...)
	described = append(described, kafkaString("billing")...)
	described = append(described, kafkaString("PreparingRebalance")...)
	described = append(described, kafkaString("consumer")...)
	described = append(described, kafkaString("range")...)
	described = append(described, appendInt32(nil, 1)...)
	described = append(described, kafkaString("m1")...)
	described = append(described, kafkaString("app")...)
	described = append(described, kafkaString("10.0.0.9")...)
	described = append(described, appendInt32(nil, 0)...)
	described = append(described, appendInt32(nil, 0)...)
	infos, err := ParseDescribeGroups(described, 0)
	if err != nil || infos["billing"].State != "PreparingRebalance" || infos["billing"].Members != 1 {
		t.Fatalf("%v %v", infos, err)
	}
	cfg := appendInt32(nil, 0)
	cfg = append(cfg, appendInt32(nil, 1)...)
	cfg = append(cfg, appendInt16(nil, 0)...)
	cfg = append(cfg, appendInt16(nil, -1)...)
	cfg = append(cfg, 4)
	cfg = append(cfg, kafkaString("1")...)
	cfg = append(cfg, appendInt32(nil, 1)...)
	cfg = append(cfg, kafkaString("min.insync.replicas")...)
	cfg = append(cfg, kafkaString("2")...)
	cfg = append(cfg, 0, 0)
	min, err := ParseMinInSync(cfg, 0)
	if err != nil || min != 2 {
		t.Fatalf("%d %v", min, err)
	}
}

func TestParseLogBytes(t *testing.T) {
	body := appendInt32(nil, 0)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt16(nil, 0)...)
	body = append(body, kafkaString("/data")...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, kafkaString("orders")...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt32(nil, 0)...)
	body = append(body, appendInt64(nil, 4096)...)
	body = append(body, appendInt64(nil, 0)...)
	body = append(body, []byte{0}...)
	n, ok := ParseLogBytes(body)
	if !ok || n != 4096 {
		t.Fatalf("%d %v", n, ok)
	}
}

func TestTopicMinOverride(t *testing.T) {
	body := appendInt32(nil, 0)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, broker(1, "a", 9092)...)
	body = append(body, appendInt16(nil, -1)...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, appendInt16(nil, 0)...)
	body = append(body, kafkaString("orders")...)
	body = append(body, []byte{0}...)
	body = append(body, appendInt32(nil, 1)...)
	body = append(body, partition(0, 0, 1, []int32{1, 2}, []int32{1, 2})...)
	body = append(body, appendInt32(nil, 0)...)
	sample, err := ParseMetadata(body, 7)
	if err != nil {
		t.Fatal(err)
	}
	sample.applyMinISR(1, map[string]int{"orders": 3})
	if sample.UnderMinIsr != 1 || sample.Topics[0].BelowMin != 1 || sample.Topics[0].Replicas != 2 {
		t.Fatalf("%+v %+v", sample, sample.Topics[0])
	}
}

func broker(id int32, host string, port int32) []byte {
	out := appendInt32(nil, id)
	out = append(out, kafkaString(host)...)
	out = append(out, appendInt32(nil, port)...)
	return append(out, appendInt16(nil, -1)...)
}

func partition(code int, id, leader int32, replicas, isr []int32) []byte {
	out := appendInt16(nil, int16(code))
	out = append(out, appendInt32(nil, id)...)
	out = append(out, appendInt32(nil, leader)...)
	out = append(out, appendInt32(nil, int32(len(replicas)))...)
	for _, item := range replicas {
		out = append(out, appendInt32(nil, item)...)
	}
	out = append(out, appendInt32(nil, int32(len(isr)))...)
	for _, item := range isr {
		out = append(out, appendInt32(nil, item)...)
	}
	return out
}
