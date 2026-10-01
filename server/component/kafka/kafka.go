// Package kafka 用 Kafka 协议读取集群健康、日志占用和消费延迟，并写入 FG_COMPONENT_KAFKA_METRIC。
// 先做 ApiVersions，再按协商到的版本读 Metadata、副本、主题最小 ISR 和消费组。
// 传输可以是明文或 TLS。认证支持不认证、SASL PLAIN、SCRAM-SHA-256 和 SCRAM-SHA-512。
package kafka

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/pane"
	"fluxguard/server/component/profile"
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

const (
	detailBrokerCap = 128
	detailTopicCap  = 250
	detailGroupCap  = 250
	topicConfigCap  = 320
)

// link 是这一轮拨号的传输和认证。sasl 为空表示不认证。
type link struct {
	user   string
	secret string
	tlsOn  bool
	sasl   string
}

func linkOf(target conn.Open) link {
	deploy, auth := profile.Split(target.OptionText)
	out := link{user: target.UserName, secret: target.Secret, tlsOn: deploy == "tls"}
	switch auth {
	case "plain":
		out.sasl = "PLAIN"
	case "scram256":
		out.sasl = "SCRAM-SHA-256"
	case "scram512":
		out.sasl = "SCRAM-SHA-512"
	}
	if out.sasl == "" && auth == "" && (out.user != "" || out.secret != "") {
		out.sasl = "PLAIN"
	}
	return out
}

// Sample 是一轮 Kafka 采集。
type Sample struct {
	BrokerCount       int64
	OnlineBrokers     int64
	OfflineBrokers    int64
	OfflinePartitions int64
	UnderReplicated   int64
	PreferredReplicas int64
	UnderMinIsr       int64
	TopicCount        int64
	GroupCount        int64
	RebalanceGroups   int64
	MaxLag            int64
	TotalLag          int64
	LogBytes          int64
	ClusterID         string
	ControllerID      int32
	Topics            []TopicStat
	Brokers           []BrokerStat
	Groups            []GroupStat
	GroupNote         string
	TopicNote         string
}

// BrokerStat 是元数据里的一台 Broker，以及从探针看它是否还连得上。
type BrokerStat struct {
	ID         int32  `json:"id"`
	Host       string `json:"host"`
	Port       int32  `json:"port"`
	Controller bool   `json:"controller"`
	Up         bool   `json:"up"`
	LogBytes   int64  `json:"logBytes"`
}

// TopicStat 是一个 Topic 的分区健康。
type TopicStat struct {
	Name       string `json:"name"`
	Partitions int    `json:"partitions"`
	Offline    int    `json:"offline"`
	Under      int    `json:"under"`
	Skew       int    `json:"skew"`
	Replicas   int    `json:"replicas,omitempty"`
	ReplicaMax int    `json:"replicaMax,omitempty"`
	BelowMin   int    `json:"belowMin,omitempty"`
	Internal   bool   `json:"internal,omitempty"`
	partIDs    []int32
	onlineISR  []int
}

// GroupStat 是一个消费组的状态和滞后。Lag 是各分区滞后合计，MaxPart 是最慢的一个分区。
type GroupStat struct {
	Name    string `json:"name"`
	Lag     int64  `json:"lag"`
	MaxPart int64  `json:"maxLag"`
	Part    int32  `json:"part"`
	Topic   string `json:"topic"`
	Topics  int    `json:"topicCount,omitempty"`
	State   string `json:"state,omitempty"`
	Members int    `json:"members"`
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
	return series.Read(ctx, db, "FG_COMPONENT_KAFKA_METRIC", tenantID, componentID, catalog.Columns(catalog.Kafka), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	addrs, err := netx.HostPorts(target.Address, 9092)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	var last error
	for _, addr := range addrs {
		sample, status, err := one(ctx, addr, linkOf(target))
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

func one(ctx context.Context, addr string, opt link) (Sample, models.Status, error) {
	raw, apis, err := connect(ctx, addr, opt)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}, err
	}
	defer raw.Close()
	ver, ok := apis.use(3, 7)
	if !ok || ver < 1 {
		return Sample{}, models.Status{Error: "Kafka 协议版本不受支持"}, fmt.Errorf("Kafka 协议版本不受支持")
	}
	body, err := roundTrip(raw, 3, ver, 4, metadataBody(ver))
	if err != nil {
		return Sample{}, models.Status{}, err
	}
	sample, err := ParseMetadata(body, ver)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}, err
	}
	var version string
	if minISR, text, cfgOK := readBrokerConfig(raw, apis, sample.minISRBroker()); cfgOK {
		version = text
		names := topicConfigOrder(sample.Topics)
		if len(names) > topicConfigCap {
			sample.TopicNote = fmt.Sprintf("主题共 %d 个，最小 ISR 这次核对了前 %d 个，其余用 Broker 默认值。", len(names), topicConfigCap)
		}
		sample.applyMinISR(minISR, readTopicMinISR(raw, apis, sample.Topics))
	}
	sample.Brokers = annotateBrokers(ctx, sample.Brokers, addr, opt)
	if note := sample.applyReach(); note != "" {
		sample.GroupNote = note
	}
	groups, groupCount, maxLag, totalLag, rebalance, note := collectGroups(ctx, addr, sample.Brokers, opt, sample.Topics, apis)
	sample.Groups = groups
	sample.GroupCount = groupCount
	sample.MaxLag = maxLag
	sample.TotalLag = totalLag
	sample.RebalanceGroups = rebalance
	if note != "" {
		if sample.GroupNote != "" {
			sample.GroupNote += "；"
		}
		sample.GroupNote += note
	}
	fillLog(ctx, &sample, opt, apis)
	summary := fmt.Sprintf("在线 %d/%d · Topic %d · 离线分区 %d", sample.OnlineBrokers, sample.BrokerCount, sample.TopicCount, sample.OfflinePartitions)
	if maxLag >= 0 {
		summary += fmt.Sprintf(" · 最大延迟 %d", maxLag)
	}
	return sample, models.Status{Up: true, Role: "cluster", Version: version, Summary: summary}, nil
}

func metadataBody(ver int16) []byte {
	body := []byte{0xff, 0xff, 0xff, 0xff}
	if ver >= 4 {
		body = append(body, 0)
	}
	return body
}

func dial(ctx context.Context, addr string, tlsOn bool) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	if !tlsOn {
		return raw, nil
	}
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		host = addr
	}
	secure := tls.Client(raw, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err := secure.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, fmt.Errorf("Kafka TLS 握手失败")
	}
	return secure, nil
}

// connect 先协商 ApiVersions，失败则退回固定旧版本。需要认证时接着做 SASL。
func connect(ctx context.Context, addr string, opt link) (net.Conn, apiSet, error) {
	raw, err := dial(ctx, addr, opt.tlsOn)
	if err != nil {
		return nil, nil, err
	}
	body, err := roundTrip(raw, 18, 0, 1, nil)
	var apis apiSet
	if err != nil {
		raw.Close()
		raw, err = dial(ctx, addr, opt.tlsOn)
		if err != nil {
			return nil, nil, err
		}
		apis = legacyAPIs()
	} else {
		apis, err = ParseApiVersions(body)
		if err != nil {
			raw.Close()
			return nil, nil, err
		}
	}
	if err := authenticate(raw, opt); err != nil {
		raw.Close()
		return nil, nil, err
	}
	return raw, apis, nil
}

func authenticate(raw net.Conn, opt link) error {
	switch opt.sasl {
	case "":
		return nil
	case "PLAIN":
		return saslPlain(raw, opt.user, opt.secret)
	case "SCRAM-SHA-256", "SCRAM-SHA-512":
		return saslScram(raw, opt.sasl, opt.user, opt.secret)
	default:
		return fmt.Errorf("不支持的 Kafka 认证方式")
	}
}

func annotateBrokers(ctx context.Context, brokers []BrokerStat, seed string, opt link) []BrokerStat {
	for i := range brokers {
		if brokerAddr(brokers[i]) == seed {
			brokers[i].Up = true
		}
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range brokers {
		if brokers[i].Up || brokers[i].Host == "" || brokers[i].Port <= 0 {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			raw, _, err := connect(probeCtx, brokerAddr(brokers[i]), opt)
			if err != nil {
				return
			}
			raw.Close()
			brokers[i].Up = true
		}(i)
	}
	wg.Wait()
	sort.Slice(brokers, func(i, j int) bool { return brokers[i].ID < brokers[j].ID })
	return brokers
}

func brokerAddr(item BrokerStat) string {
	return net.JoinHostPort(item.Host, strconv.Itoa(int(item.Port)))
}

func (s *Sample) minISRBroker() int32 {
	if s.ControllerID >= 0 {
		return s.ControllerID
	}
	if len(s.Brokers) > 0 {
		return s.Brokers[0].ID
	}
	return 0
}

func (s *Sample) applyReach() string {
	var up int64
	for _, item := range s.Brokers {
		if item.Up {
			up++
		}
	}
	if up == 0 && s.BrokerCount > 0 {
		s.OnlineBrokers = 1
		s.OfflineBrokers = 0
		return "已读到集群元数据，但公告的 Broker 地址从这里连不上，所以没有把它们记成离线。"
	}
	s.OnlineBrokers = up
	s.OfflineBrokers = s.BrokerCount - up
	if s.OfflineBrokers < 0 {
		s.OfflineBrokers = 0
	}
	return ""
}

func (s *Sample) applyMinISR(fallback int, overrides map[string]int) {
	if fallback < 1 && len(overrides) == 0 {
		s.UnderMinIsr = -1
		return
	}
	var n int64
	for i := range s.Topics {
		topic := &s.Topics[i]
		min := fallback
		if v, ok := overrides[topic.Name]; ok && v > 0 {
			min = v
		}
		if min < 1 {
			continue
		}
		for _, isr := range topic.onlineISR {
			if isr < min {
				n++
				topic.BelowMin++
			}
		}
	}
	s.UnderMinIsr = n
}

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

// ParseMetadata 解析 Metadata v1 到 v7 的应答正文（不含关联号）。
func ParseMetadata(body []byte, ver int16) (Sample, error) {
	r := reader{b: body}
	if ver >= 3 {
		if _, err := r.i32(); err != nil {
			return Sample{}, err
		}
	}
	brokerN, err := r.i32()
	if err != nil || brokerN < 0 {
		return Sample{}, fmt.Errorf("Kafka 应答无效")
	}
	listedBrokers := make([]BrokerStat, 0, brokerN)
	for i := int32(0); i < brokerN; i++ {
		id, err := r.i32()
		if err != nil {
			return Sample{}, err
		}
		host, err := r.str()
		if err != nil {
			return Sample{}, err
		}
		port, err := r.i32()
		if err != nil {
			return Sample{}, err
		}
		if _, _, err := r.nullable(); err != nil {
			return Sample{}, err
		}
		listedBrokers = append(listedBrokers, BrokerStat{ID: id, Host: host, Port: port, LogBytes: -1})
	}
	var clusterID string
	if ver >= 2 {
		clusterID, _, err = r.nullable()
		if err != nil {
			return Sample{}, err
		}
	}
	controller, err := r.i32()
	if err != nil {
		return Sample{}, err
	}
	for i := range listedBrokers {
		listedBrokers[i].Controller = listedBrokers[i].ID == controller
	}
	topics, err := r.i32()
	if err != nil || topics < 0 {
		return Sample{}, fmt.Errorf("Kafka 应答无效")
	}
	var offline, under, preferred, userTopics int64
	var listed []TopicStat
	for i := int32(0); i < topics; i++ {
		if _, err := r.i16(); err != nil {
			return Sample{}, err
		}
		name, err := r.str()
		if err != nil {
			return Sample{}, err
		}
		internal, err := r.i8()
		if err != nil {
			return Sample{}, err
		}
		parts, err := r.i32()
		if err != nil || parts < 0 {
			return Sample{}, fmt.Errorf("Kafka 应答无效")
		}
		topic := TopicStat{Name: name, Partitions: int(parts), Internal: internal != 0}
		if !topic.Internal {
			userTopics++
		}
		var sawReplica bool
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
			if ver >= 7 {
				if _, err := r.i32s(); err != nil {
					return Sample{}, err
				}
			}
			if nrep := len(replicas); nrep > 0 {
				if !sawReplica || nrep < topic.Replicas {
					topic.Replicas = nrep
				}
				if nrep > topic.ReplicaMax {
					topic.ReplicaMax = nrep
				}
				sawReplica = true
			}
			if code != 0 || leader < 0 {
				offline++
				topic.Offline++
				continue
			}
			topic.onlineISR = append(topic.onlineISR, len(isr))
			if len(replicas) > len(isr) {
				under++
				topic.Under++
			}
			if len(replicas) > 0 && leader != replicas[0] {
				preferred++
				topic.Skew++
			}
		}
		listed = append(listed, topic)
	}
	return Sample{
		BrokerCount: int64(brokerN), OfflinePartitions: offline, UnderReplicated: under,
		PreferredReplicas: preferred, UnderMinIsr: -1, TopicCount: userTopics,
		Topics: listed, Brokers: listedBrokers, ClusterID: clusterID, ControllerID: controller,
		MaxLag: -1, TotalLag: -1, LogBytes: -1,
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

func (r *reader) skipBytes() error {
	n, err := r.i32()
	if err != nil || n < -1 {
		return fmt.Errorf("Kafka 应答无效")
	}
	if n <= 0 {
		return nil
	}
	_, err = r.take(int(n))
	return err
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
	topics, topicList := presentTopics(sample.Topics)
	groups, groupList := presentGroups(sample.Groups)
	brokers, brokerNote := presentBrokers(sample.Brokers)
	topicNote := joinNote(sample.TopicNote, topicList)
	groupNote := joinNote(sample.GroupNote, groupList)
	if len(topics) == 0 && len(groups) == 0 && len(brokers) == 0 && topicNote == "" && groupNote == "" && brokerNote == "" && sample.ClusterID == "" {
		return ""
	}
	body, err := json.Marshal(struct {
		ClusterID  string       `json:"clusterId,omitempty"`
		Brokers    []BrokerStat `json:"brokers,omitempty"`
		Topics     []TopicStat  `json:"topics,omitempty"`
		Groups     []GroupStat  `json:"groups,omitempty"`
		GroupNote  string       `json:"groupNote,omitempty"`
		TopicNote  string       `json:"topicNote,omitempty"`
		BrokerNote string       `json:"brokerNote,omitempty"`
	}{
		ClusterID: sample.ClusterID, Brokers: brokers, Topics: topics, Groups: groups,
		GroupNote: groupNote, TopicNote: topicNote, BrokerNote: brokerNote,
	})
	if err != nil {
		return ""
	}
	return string(body)
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "；" + b
}

func presentBrokers(in []BrokerStat) ([]BrokerStat, string) {
	out := append([]BrokerStat(nil), in...)
	if len(out) <= detailBrokerCap {
		return out, ""
	}
	return out[:detailBrokerCap], fmt.Sprintf("Broker 共 %d 台，这里列出前 %d 台。", len(out), detailBrokerCap)
}

func presentTopics(in []TopicStat) ([]TopicStat, string) {
	out := make([]TopicStat, 0, len(in))
	for _, item := range in {
		if item.Internal && item.Offline == 0 && item.Under == 0 && item.Skew == 0 && item.BelowMin == 0 {
			continue
		}
		if item.ReplicaMax == item.Replicas {
			item.ReplicaMax = 0
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Offline != out[j].Offline {
			return out[i].Offline > out[j].Offline
		}
		if out[i].Under != out[j].Under {
			return out[i].Under > out[j].Under
		}
		if out[i].BelowMin != out[j].BelowMin {
			return out[i].BelowMin > out[j].BelowMin
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > detailTopicCap {
		note := fmt.Sprintf("主题共 %d 个，这里列出前 %d 个，优先离线和未同步。", len(out), detailTopicCap)
		return out[:detailTopicCap], note
	}
	return out, ""
}

func presentGroups(in []GroupStat) ([]GroupStat, string) {
	out := append([]GroupStat(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if (out[i].MaxPart < 0) != (out[j].MaxPart < 0) {
			return out[i].MaxPart >= 0
		}
		if out[i].MaxPart != out[j].MaxPart {
			return out[i].MaxPart > out[j].MaxPart
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > detailGroupCap {
		note := fmt.Sprintf("消费组共 %d 个，这里列出延迟最高的前 %d 个。", len(out), detailGroupCap)
		return out[:detailGroupCap], note
	}
	return out, ""
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
		"brokerCount": sample.BrokerCount, "onlineBrokers": sample.OnlineBrokers,
		"offlineBrokers": sample.OfflineBrokers, "offlinePartitions": sample.OfflinePartitions,
		"underReplicated": sample.UnderReplicated, "preferredReplicas": sample.PreferredReplicas,
		"underMinIsr": sample.UnderMinIsr, "topicCount": sample.TopicCount,
		"groupCount": sample.GroupCount, "rebalanceGroups": sample.RebalanceGroups,
		"maxLag": sample.MaxLag, "totalLag": sample.TotalLag, "logBytes": sample.LogBytes, "extProperty": kafkaDetail(sample),
	}
	if !status.Up {
		args["maxLag"] = -1
		args["totalLag"] = -1
		args["underMinIsr"] = -1
		args["logBytes"] = -1
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_COMPONENT_KAFKA_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  brokerCount, onlineBrokers, offlineBrokers, offlinePartitions, underReplicated, preferredReplicas, underMinIsr,
		  topicCount, groupCount, rebalanceGroups, maxLag, totalLag, logBytes, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @brokerCount, @onlineBrokers, @offlineBrokers, @offlinePartitions, @underReplicated, @preferredReplicas, @underMinIsr,
		  @topicCount, @groupCount, @rebalanceGroups, @maxLag, @totalLag, @logBytes, @extProperty,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	if err != nil {
		return err
	}
	return pane.Touch(ctx, db, catalog.Kafka, args)
}
