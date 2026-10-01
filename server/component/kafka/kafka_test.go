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
	sample, err := ParseMetadata(body)
	if err != nil {
		t.Fatal(err)
	}
	if sample.BrokerCount != 2 || sample.OfflinePartitions != 1 || sample.UnderReplicated != 1 {
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
