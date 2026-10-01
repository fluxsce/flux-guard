package kafka

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

func collectGroups(ctx context.Context, addr, user, secret string, topics []TopicStat) ([]GroupStat, int64, string) {
	if len(topics) == 0 {
		return nil, -1, ""
	}
	raw, err := openBroker(ctx, addr, user, secret)
	if err != nil {
		return nil, -1, "消费组暂时读不到"
	}
	defer raw.Close()
	body, err := roundTrip(raw, 16, 0, 20, nil)
	if err != nil {
		return nil, -1, "消费组暂时读不到"
	}
	names, err := ParseListGroups(body)
	if err != nil {
		return nil, -1, err.Error()
	}
	if len(names) == 0 {
		return nil, -1, ""
	}
	if len(names) > 8 {
		names = names[:8]
	}
	var out []GroupStat
	maxLag := int64(-1)
	for i, name := range names {
		body, err := roundTrip(raw, 10, 0, int32(30+i), kafkaString(name))
		if err != nil {
			out = append(out, GroupStat{Name: name, Lag: -1})
			continue
		}
		host, port, err := ParseCoordinator(body)
		if err != nil {
			out = append(out, GroupStat{Name: name, Lag: -1})
			continue
		}
		lag, topic := groupLag(ctx, net.JoinHostPort(host, strconv.Itoa(int(port))), user, secret, name, topics)
		out = append(out, GroupStat{Name: name, Lag: lag, Topic: topic})
		if lag > maxLag {
			maxLag = lag
		}
	}
	return out, maxLag, ""
}

func groupLag(ctx context.Context, addr, user, secret, group string, topics []TopicStat) (int64, string) {
	raw, err := openBroker(ctx, addr, user, secret)
	if err != nil {
		return -1, ""
	}
	defer raw.Close()
	picked := topics
	if len(picked) > 16 {
		picked = picked[:16]
	}
	fetched, err := roundTrip(raw, 9, 1, 40, offsetFetchBody(group, picked))
	if err != nil {
		return -1, ""
	}
	committed, err := ParseOffsetFetch(fetched)
	if err != nil {
		return -1, ""
	}
	endsBody, err := roundTrip(raw, 2, 1, 41, listOffsetsBody(picked))
	if err != nil {
		return -1, ""
	}
	ends, err := ParseListOffsets(endsBody)
	if err != nil {
		return -1, ""
	}
	var total int64
	var top string
	var topLag int64
	var seen bool
	for _, topic := range picked {
		var topicLag int64
		var topicSeen bool
		for _, part := range topic.partIDs {
			commit, ok := committed[topic.Name][part]
			end, endOK := ends[topic.Name][part]
			if !ok || !endOK || commit < 0 || end < 0 {
				continue
			}
			lag := end - commit
			if lag < 0 {
				lag = 0
			}
			topicLag += lag
			topicSeen = true
		}
		if !topicSeen {
			continue
		}
		seen = true
		total += topicLag
		if topicLag >= topLag {
			topLag = topicLag
			top = topic.Name
		}
	}
	if !seen {
		return -1, ""
	}
	return total, top
}

func openBroker(ctx context.Context, addr, user, secret string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	if user != "" || secret != "" {
		if err := saslPlain(raw, user, secret); err != nil {
			raw.Close()
			return nil, err
		}
	}
	return raw, nil
}

func offsetFetchBody(group string, topics []TopicStat) []byte {
	body := kafkaString(group)
	body = appendInt32(body, int32(len(topics)))
	for _, topic := range topics {
		parts := topic.partIDs
		if len(parts) > 32 {
			parts = parts[:32]
		}
		body = append(body, kafkaString(topic.Name)...)
		body = appendInt32(body, int32(len(parts)))
		for _, part := range parts {
			body = appendInt32(body, part)
		}
	}
	return body
}

func listOffsetsBody(topics []TopicStat) []byte {
	body := appendInt32(nil, -1)
	body = appendInt32(body, int32(len(topics)))
	for _, topic := range topics {
		parts := topic.partIDs
		if len(parts) > 32 {
			parts = parts[:32]
		}
		body = append(body, kafkaString(topic.Name)...)
		body = appendInt32(body, int32(len(parts)))
		for _, part := range parts {
			body = appendInt32(body, part)
			body = appendInt64(body, -1)
		}
	}
	return body
}

// ParseListGroups 解析 ListGroups v0。
func ParseListGroups(body []byte) ([]string, error) {
	r := reader{b: body}
	code, err := r.i16()
	if err != nil || code != 0 {
		return nil, fmt.Errorf("Kafka 消费组列表失败")
	}
	n, err := r.i32()
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := make([]string, 0, n)
	for i := int32(0); i < n; i++ {
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		if _, err := r.str(); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, nil
}

// ParseCoordinator 解析 FindCoordinator v0。
func ParseCoordinator(body []byte) (string, int32, error) {
	r := reader{b: body}
	code, err := r.i16()
	if err != nil || code != 0 {
		return "", 0, fmt.Errorf("Kafka 找不到消费组协调者")
	}
	if _, err := r.i32(); err != nil {
		return "", 0, err
	}
	host, err := r.str()
	if err != nil {
		return "", 0, err
	}
	port, err := r.i32()
	if err != nil || host == "" || port <= 0 {
		return "", 0, fmt.Errorf("Kafka 找不到消费组协调者")
	}
	return host, port, nil
}

// ParseOffsetFetch 解析 OffsetFetch v1，返回 topic、分区到已提交位移。
func ParseOffsetFetch(body []byte) (map[string]map[int32]int64, error) {
	r := reader{b: body}
	topics, err := r.i32()
	if err != nil || topics < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := map[string]map[int32]int64{}
	for i := int32(0); i < topics; i++ {
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		parts, err := r.i32()
		if err != nil || parts < 0 {
			return nil, fmt.Errorf("Kafka 应答无效")
		}
		out[name] = map[int32]int64{}
		for j := int32(0); j < parts; j++ {
			part, err := r.i32()
			if err != nil {
				return nil, err
			}
			offset, err := r.i64()
			if err != nil {
				return nil, err
			}
			if _, _, err := r.nullable(); err != nil {
				return nil, err
			}
			if _, err := r.i16(); err != nil {
				return nil, err
			}
			out[name][part] = offset
		}
	}
	return out, nil
}

// ParseListOffsets 解析 ListOffsets v1，返回 topic、分区到日志末端。
func ParseListOffsets(body []byte) (map[string]map[int32]int64, error) {
	r := reader{b: body}
	topics, err := r.i32()
	if err != nil || topics < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := map[string]map[int32]int64{}
	for i := int32(0); i < topics; i++ {
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		parts, err := r.i32()
		if err != nil || parts < 0 {
			return nil, fmt.Errorf("Kafka 应答无效")
		}
		out[name] = map[int32]int64{}
		for j := int32(0); j < parts; j++ {
			part, err := r.i32()
			if err != nil {
				return nil, err
			}
			code, err := r.i16()
			if err != nil {
				return nil, err
			}
			if _, err := r.i64(); err != nil {
				return nil, err
			}
			offset, err := r.i64()
			if err != nil {
				return nil, err
			}
			if code == 0 {
				out[name][part] = offset
			}
		}
	}
	return out, nil
}
