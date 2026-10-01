package kafka

import (
	"fmt"
	"net"
	"sort"
	"strconv"
)

type apiSet map[int16][2]int16

func legacyAPIs() apiSet {
	return apiSet{
		2:  {1, 1},
		3:  {1, 1},
		9:  {1, 1},
		10: {0, 0},
		15: {0, 0},
		16: {0, 0},
		17: {1, 1},
		32: {0, 0},
		36: {0, 0},
	}
}

// use 选取不超过 max、且落在 broker 支持范围内的最高版本。
func (a apiSet) use(key, max int16) (int16, bool) {
	if a == nil {
		return 0, false
	}
	span, ok := a[key]
	if !ok || max < span[0] || span[1] < span[0] {
		return 0, false
	}
	if span[1] < max {
		return span[1], true
	}
	return max, true
}

// ParseApiVersions 解析 ApiVersions v0。
func ParseApiVersions(body []byte) (apiSet, error) {
	r := reader{b: body}
	code, err := r.i16()
	if err != nil || code != 0 {
		return nil, fmt.Errorf("Kafka 协议协商失败")
	}
	n, err := r.i32()
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := apiSet{}
	for i := int32(0); i < n; i++ {
		key, err := r.i16()
		if err != nil {
			return nil, err
		}
		minV, err := r.i16()
		if err != nil {
			return nil, err
		}
		maxV, err := r.i16()
		if err != nil {
			return nil, err
		}
		out[int16(key)] = [2]int16{int16(minV), int16(maxV)}
	}
	return out, nil
}

func readBrokerConfig(raw net.Conn, apis apiSet, brokerID int32) (min int, version string, ok bool) {
	ver, okVer := apis.use(32, 1)
	if !okVer {
		ver, okVer = apis.use(32, 0)
		if !okVer {
			return 0, "", false
		}
	}
	body, err := roundTrip(raw, 32, ver, 7, describeBrokerBody(ver, brokerID))
	if err != nil {
		return 0, "", false
	}
	parsed, err := ParseConfigValues(body, ver)
	if err != nil || len(parsed) == 0 {
		return 0, "", false
	}
	var got bool
	for _, item := range parsed {
		if n, err := strconv.Atoi(item.Values["min.insync.replicas"]); err == nil && n > 0 {
			min = n
			got = true
		}
		if text := item.Values["inter.broker.protocol.version"]; text != "" {
			version = text
			got = true
		}
	}
	return min, version, got
}

func readTopicMinISR(raw net.Conn, apis apiSet, topics []TopicStat) map[string]int {
	ver, ok := apis.use(32, 1)
	if !ok {
		ver, ok = apis.use(32, 0)
		if !ok {
			return nil
		}
	}
	names := topicConfigOrder(topics)
	if len(names) > topicConfigCap {
		names = names[:topicConfigCap]
	}
	out := map[string]int{}
	corr := int32(11)
	for _, chunk := range chunks(names, 40) {
		body, err := roundTrip(raw, 32, ver, corr, describeTopicsBody(ver, chunk))
		corr++
		if err != nil {
			return out
		}
		parsed, err := ParseConfigValues(body, ver)
		if err != nil {
			return out
		}
		for _, item := range parsed {
			n, err := strconv.Atoi(item.Values["min.insync.replicas"])
			if err != nil || n < 1 {
				continue
			}
			out[item.Name] = n
		}
	}
	return out
}

func topicConfigOrder(topics []TopicStat) []string {
	sorted := append([]TopicStat(nil), topics...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Internal != sorted[j].Internal {
			return !sorted[i].Internal
		}
		if sorted[i].Offline != sorted[j].Offline {
			return sorted[i].Offline > sorted[j].Offline
		}
		if sorted[i].Under != sorted[j].Under {
			return sorted[i].Under > sorted[j].Under
		}
		return sorted[i].Name < sorted[j].Name
	})
	out := make([]string, 0, len(sorted))
	for _, topic := range sorted {
		if topic.Internal || topic.Name == "" {
			continue
		}
		out = append(out, topic.Name)
	}
	return out
}

func describeBrokerBody(ver int16, brokerID int32) []byte {
	body := appendInt32(nil, 1)
	body = append(body, 4)
	body = append(body, kafkaString(strconv.FormatInt(int64(brokerID), 10))...)
	body = appendInt32(body, 2)
	body = append(body, kafkaString("min.insync.replicas")...)
	body = append(body, kafkaString("inter.broker.protocol.version")...)
	if ver >= 1 {
		body = append(body, 0)
	}
	return body
}

func describeTopicsBody(ver int16, names []string) []byte {
	body := appendInt32(nil, int32(len(names)))
	for _, name := range names {
		body = append(body, 2)
		body = append(body, kafkaString(name)...)
		body = appendInt32(body, 1)
		body = append(body, kafkaString("min.insync.replicas")...)
	}
	if ver >= 1 {
		body = append(body, 0)
	}
	return body
}

// configResult 是 DescribeConfigs 里一项资源读到的配置。
type configResult struct {
	Name   string
	Values map[string]string
}

// ParseConfigValues 解析 DescribeConfigs v0 或 v1，跳过失败的资源。
func ParseConfigValues(body []byte, ver int16) ([]configResult, error) {
	r := reader{b: body}
	if _, err := r.i32(); err != nil {
		return nil, err
	}
	n, err := r.i32()
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := make([]configResult, 0, n)
	for i := int32(0); i < n; i++ {
		code, err := r.i16()
		if err != nil {
			return nil, err
		}
		if _, _, err := r.nullable(); err != nil {
			return nil, err
		}
		if _, err := r.i8(); err != nil {
			return nil, err
		}
		name, err := r.str()
		if err != nil {
			return nil, err
		}
		configs, err := r.i32()
		if err != nil || configs < 0 {
			return nil, fmt.Errorf("Kafka 应答无效")
		}
		values := map[string]string{}
		for c := int32(0); c < configs; c++ {
			key, err := r.str()
			if err != nil {
				return nil, err
			}
			value, _, err := r.nullable()
			if err != nil {
				return nil, err
			}
			if _, err := r.i8(); err != nil {
				return nil, err
			}
			if ver >= 1 {
				if _, err := r.i8(); err != nil {
					return nil, err
				}
			}
			if _, err := r.i8(); err != nil {
				return nil, err
			}
			if ver >= 1 {
				syn, err := r.i32()
				if err != nil || syn < 0 {
					return nil, fmt.Errorf("Kafka 应答无效")
				}
				for s := int32(0); s < syn; s++ {
					if _, err := r.str(); err != nil {
						return nil, err
					}
					if _, _, err := r.nullable(); err != nil {
						return nil, err
					}
					if _, err := r.i8(); err != nil {
						return nil, err
					}
				}
			}
			if code == 0 {
				values[key] = value
			}
		}
		if code == 0 {
			out = append(out, configResult{Name: name, Values: values})
		}
	}
	return out, nil
}

// ParseMinInSync 从 DescribeConfigs 应答里取出 min.insync.replicas。
func ParseMinInSync(body []byte, ver int16) (int, error) {
	r := reader{b: body}
	if _, err := r.i32(); err != nil {
		return 0, err
	}
	n, err := r.i32()
	if err != nil || n < 1 {
		return 0, fmt.Errorf("Kafka 应答无效")
	}
	code, err := r.i16()
	if err != nil || code != 0 {
		return 0, fmt.Errorf("Kafka 应答无效")
	}
	if _, _, err := r.nullable(); err != nil {
		return 0, err
	}
	if _, err := r.i8(); err != nil {
		return 0, err
	}
	if _, err := r.str(); err != nil {
		return 0, err
	}
	configs, err := r.i32()
	if err != nil || configs < 0 {
		return 0, fmt.Errorf("Kafka 应答无效")
	}
	for i := int32(0); i < configs; i++ {
		name, err := r.str()
		if err != nil {
			return 0, err
		}
		value, _, err := r.nullable()
		if err != nil {
			return 0, err
		}
		if _, err := r.i8(); err != nil {
			return 0, err
		}
		if ver >= 1 {
			if _, err := r.i8(); err != nil {
				return 0, err
			}
		}
		if _, err := r.i8(); err != nil {
			return 0, err
		}
		if ver >= 1 {
			syn, err := r.i32()
			if err != nil || syn < 0 {
				return 0, fmt.Errorf("Kafka 应答无效")
			}
			for s := int32(0); s < syn; s++ {
				if _, err := r.str(); err != nil {
					return 0, err
				}
				if _, _, err := r.nullable(); err != nil {
					return 0, err
				}
				if _, err := r.i8(); err != nil {
					return 0, err
				}
			}
		}
		if name == "min.insync.replicas" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return 0, err
			}
			return parsed, nil
		}
	}
	return 0, fmt.Errorf("Kafka 应答无效")
}
