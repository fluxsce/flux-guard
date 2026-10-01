// Package kafka 用 Metadata 请求读取集群概况，并写入 FG_KAFKA_METRIC。
// 填写用户名时先做 SASL PLAIN。
package kafka

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	runtime.Register(catalog.Kafka, probe{})
}

type probe struct{}

// Sample 是一轮 Kafka 采集。
type Sample struct {
	BrokerCount       int64
	OfflinePartitions int64
	UnderReplicated   int64
	TopicCount        int64
	GroupCount        int64
	MaxLag            int64
	Topics            []TopicStat
	Groups            []GroupStat
	GroupNote         string
}

// TopicStat 是一个 Topic 的分区健康。
type TopicStat struct {
	Name       string `json:"name"`
	Partitions int    `json:"partitions"`
	Offline    int    `json:"offline"`
	Under      int    `json:"under"`
	partIDs    []int32
}

// GroupStat 是一个消费组相对日志末端的延迟。
type GroupStat struct {
	Name  string `json:"name"`
	Lag   int64  `json:"lag"`
	Topic string `json:"topic"`
}

func (probe) Probe(ctx context.Context, db *dbx.DB, target conn.Open) (models.Status, error) {
	sample, status := collect(ctx, target)
	if db != nil {
		if err := save(ctx, db, target.TenantID, target.ComponentID, sample, status); err != nil {
			return status, err
		}
	}
	return status, nil
}

func (probe) Series(ctx context.Context, db *dbx.DB, tenantID, componentID string, limit int) ([]models.Point, error) {
	return series.Read(ctx, db, "FG_KAFKA_METRIC", tenantID, componentID, catalog.Columns(catalog.Kafka), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	addrs, err := netx.HostPorts(target.Address, 9092)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	var last error
	for _, addr := range addrs {
		sample, status, err := one(ctx, addr, target.UserName, target.Secret)
		if err == nil && status.Up {
			return sample, status
		}
		if err != nil {
			last = err
			status.Error = err.Error()
		}
		if status.Error != "" {
			last = fmt.Errorf("%s", status.Error)
		}
	}
	if last == nil {
		last = fmt.Errorf("Kafka 没有可用的 broker")
	}
	return Sample{}, models.Status{Error: last.Error()}
}

func one(ctx context.Context, addr, user, secret string) (Sample, models.Status, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Sample{}, models.Status{}, err
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	if user != "" || secret != "" {
		if err := saslPlain(raw, user, secret); err != nil {
			return Sample{}, models.Status{Error: err.Error()}, err
		}
	}
	body, err := roundTrip(raw, 3, 1, 1, topicsAll)
	if err != nil {
		return Sample{}, models.Status{}, err
	}
	sample, err := ParseMetadata(body)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}, err
	}
	groups, maxLag, note := collectGroups(ctx, addr, user, secret, sample.Topics)
	sample.Groups = groups
	sample.GroupCount = int64(len(groups))
	sample.MaxLag = maxLag
	sample.GroupNote = note
	summary := fmt.Sprintf("Broker %d · Topic %d · 离线分区 %d", sample.BrokerCount, sample.TopicCount, sample.OfflinePartitions)
	if maxLag >= 0 {
		summary += fmt.Sprintf(" · 最大延迟 %d", maxLag)
	}
	return sample, models.Status{Up: true, Role: "cluster", Summary: summary}, nil
}

var topicsAll = []byte{0xff, 0xff, 0xff, 0xff}

func saslPlain(raw net.Conn, user, secret string) error {
	if _, err := roundTrip(raw, 17, 1, 2, kafkaString("PLAIN")); err != nil {
		return fmt.Errorf("Kafka 认证失败")
	}
	token := append([]byte{0}, []byte(user)...)
	token = append(token, 0)
	token = append(token, []byte(secret)...)
	body, err := roundTrip(raw, 36, 0, 3, kafkaBytes(token))
	if err != nil {
		return fmt.Errorf("Kafka 认证失败")
	}
	if len(body) < 2 {
		return fmt.Errorf("Kafka 认证失败")
	}
	if binary.BigEndian.Uint16(body[:2]) != 0 {
		return fmt.Errorf("Kafka 认证失败")
	}
	return nil
}

func roundTrip(raw net.Conn, apiKey, apiVer int16, corr int32, body []byte) ([]byte, error) {
	client := kafkaString("fluxguard")
	payload := make([]byte, 0, 8+len(client)+len(body))
	payload = appendInt16(payload, apiKey)
	payload = appendInt16(payload, apiVer)
	payload = appendInt32(payload, corr)
	payload = append(payload, client...)
	payload = append(payload, body...)
	frame := appendInt32(nil, int32(len(payload)))
	frame = append(frame, payload...)
	if _, err := raw.Write(frame); err != nil {
		return nil, err
	}
	var size int32
	if err := binary.Read(raw, binary.BigEndian, &size); err != nil {
		return nil, err
	}
	if size < 4 || size > 8*1024*1024 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(raw, buf); err != nil {
		return nil, err
	}
	return buf[4:], nil
}

// ParseMetadata 解析 Metadata v1 应答正文（不含关联号）。
func ParseMetadata(body []byte) (Sample, error) {
	r := reader{b: body}
	brokers, err := r.i32()
	if err != nil || brokers < 0 {
		return Sample{}, fmt.Errorf("Kafka 应答无效")
	}
	for i := int32(0); i < brokers; i++ {
		if _, err := r.i32(); err != nil {
			return Sample{}, err
		}
		if _, err := r.str(); err != nil {
			return Sample{}, err
		}
		if _, err := r.i32(); err != nil {
			return Sample{}, err
		}
		if _, _, err := r.nullable(); err != nil {
			return Sample{}, err
		}
	}
	if _, err := r.i32(); err != nil {
		return Sample{}, err
	}
	topics, err := r.i32()
	if err != nil || topics < 0 {
		return Sample{}, fmt.Errorf("Kafka 应答无效")
	}
	var offline, under int64
	var listed []TopicStat
	for i := int32(0); i < topics; i++ {
		if _, err := r.i16(); err != nil {
			return Sample{}, err
		}
		name, err := r.str()
		if err != nil {
			return Sample{}, err
		}
		if _, err := r.i8(); err != nil {
			return Sample{}, err
		}
		parts, err := r.i32()
		if err != nil || parts < 0 {
			return Sample{}, fmt.Errorf("Kafka 应答无效")
		}
		topic := TopicStat{Name: name, Partitions: int(parts)}
		for j := int32(0); j < parts; j++ {
			code, err := r.i16()
			if err != nil {
				return Sample{}, err
			}
			partID, err := r.i32()
			if err != nil {
				return Sample{}, err
			}
			topic.partIDs = append(topic.partIDs, partID)
			leader, err := r.i32()
			if err != nil {
				return Sample{}, err
			}
			replicas, err := r.i32s()
			if err != nil {
				return Sample{}, err
			}
			isr, err := r.i32s()
			if err != nil {
				return Sample{}, err
			}
			if code != 0 || leader < 0 {
				offline++
				topic.Offline++
			} else if len(replicas) > len(isr) {
				under++
				topic.Under++
			}
		}
		if len(listed) < 40 {
			listed = append(listed, topic)
		}
	}
	return Sample{
		BrokerCount: int64(brokers), OfflinePartitions: offline, UnderReplicated: under,
		TopicCount: int64(topics), Topics: listed, MaxLag: -1,
	}, nil
}

type reader struct {
	b []byte
	i int
}

func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || r.i+n > len(r.b) {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := r.b[r.i : r.i+n]
	r.i += n
	return out, nil
}

func (r *reader) i8() (int, error) {
	b, err := r.take(1)
	if err != nil {
		return 0, err
	}
	return int(b[0]), nil
}

func (r *reader) i16() (int, error) {
	b, err := r.take(2)
	if err != nil {
		return 0, err
	}
	return int(int16(binary.BigEndian.Uint16(b))), nil
}

func (r *reader) i64() (int64, error) {
	b, err := r.take(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b)), nil
}

func (r *reader) i32() (int32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(b)), nil
}

func (r *reader) str() (string, error) {
	n, err := r.i16()
	if err != nil {
		return "", err
	}
	if n < 0 {
		return "", fmt.Errorf("Kafka 应答无效")
	}
	b, err := r.take(n)
	return string(b), err
}

func (r *reader) nullable() (string, bool, error) {
	n, err := r.i16()
	if err != nil {
		return "", false, err
	}
	if n < 0 {
		return "", false, nil
	}
	b, err := r.take(n)
	return string(b), true, err
}

func (r *reader) i32s() ([]int32, error) {
	n, err := r.i32()
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Kafka 应答无效")
	}
	out := make([]int32, n)
	for i := range out {
		out[i], err = r.i32()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func appendInt16(b []byte, v int16) []byte {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(v))
	return append(b, buf[:]...)
}

func appendInt64(b []byte, v int64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(v))
	return append(b, buf[:]...)
}

func appendInt32(b []byte, v int32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(v))
	return append(b, buf[:]...)
}

func kafkaString(v string) []byte {
	return append(appendInt16(nil, int16(len(v))), []byte(v)...)
}

func kafkaDetail(sample Sample) string {
	if len(sample.Topics) == 0 && len(sample.Groups) == 0 && sample.GroupNote == "" {
		return ""
	}
	body, err := json.Marshal(struct {
		Topics    []TopicStat `json:"topics,omitempty"`
		Groups    []GroupStat `json:"groups,omitempty"`
		GroupNote string      `json:"groupNote,omitempty"`
	}{Topics: sample.Topics, Groups: sample.Groups, GroupNote: sample.GroupNote})
	if err != nil {
		return ""
	}
	return string(body)
}

func kafkaBytes(v []byte) []byte {
	return append(appendInt32(nil, int32(len(v))), v...)
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(time.Now().UTC()),
		"brokerCount": sample.BrokerCount, "offlinePartitions": sample.OfflinePartitions,
		"underReplicated": sample.UnderReplicated, "topicCount": sample.TopicCount,
		"groupCount": sample.GroupCount, "maxLag": sample.MaxLag, "extProperty": kafkaDetail(sample),
	}
	if !status.Up {
		args["maxLag"] = -1
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_KAFKA_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  brokerCount, offlinePartitions, underReplicated, topicCount, groupCount, maxLag, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @brokerCount, @offlinePartitions, @underReplicated, @topicCount, @groupCount, @maxLag, @extProperty,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	return err
}
