// Package redis 用 RESP 读取 INFO，并写入 FG_REDIS_METRIC。
package redis

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/profile"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	runtime.Register(catalog.Redis, probe{})
}

type probe struct{}

// Sample 是一轮 Redis 采集。
// 命中率、淘汰、过期、拒绝连接在入库前按上一采集点改成区间值；ParseInfo 里的命中率仍是启动以来的累计值。
type Sample struct {
	UsedMemoryBytes     int64
	MaxMemoryBytes      int64
	MemUsagePercent     float64
	UsedMemoryPeakBytes int64
	ConnectedClients    int64
	BlockedClients      int64
	OpsPerSec           float64
	HitRate             float64
	NetInKbps           float64
	NetOutKbps          float64
	ConnectedSlaves     int64
	KnownNodes          int64
	FailSlots           int64
	MemFragmentation    float64
	KeyCount            int64
	SlowlogLen          int64
	UptimeSec           int64
	ReplicaLagSec       float64
	MasterLinkDown      float64
	RdbOk               float64
	AofOk               float64
	EvictedPerSec       float64
	ExpiredPerSec       float64
	RejectedPerSec      float64
	KeyspaceHits        int64
	KeyspaceMisses      int64
	EvictedKeys         int64
	ExpiredKeys         int64
	RejectedConnections int64
	Version             string
	Role                string
	DBs                 []DbStat
	Slow                []SlowOp
}

// DbStat 是一个库的键数量。
type DbStat struct {
	Name    string `json:"name"`
	Keys    int64  `json:"keys"`
	Expires int64  `json:"expires"`
}

// SlowOp 是慢操作日志里的一条。
type SlowOp struct {
	Usec    int64  `json:"usec"`
	Command string `json:"command"`
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
	return series.Read(ctx, db, "FG_REDIS_METRIC", tenantID, componentID, catalog.Columns(catalog.Redis), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	deploy, auth := profile.Split(target.OptionText)
	if auth == "" {
		auth = inferredAuth(target)
	}
	switch deploy {
	case "", "standalone":
		return collectNode(ctx, target, 6379, auth, false)
	case "cluster":
		return collectCluster(ctx, target, auth)
	case "sentinel":
		return collectSentinel(ctx, target, auth)
	default:
		return Sample{HitRate: -1}, models.Status{Error: "不支持的 Redis 部署方式"}
	}
}

func inferredAuth(target conn.Open) string {
	if target.UserName != "" {
		return "acl"
	}
	if target.Secret != "" {
		return "password"
	}
	return "none"
}

func collectNode(ctx context.Context, target conn.Open, port int, auth string, allowMany bool) (Sample, models.Status) {
	var addrs []string
	var err error
	if allowMany {
		addrs, err = netx.HostPorts(target.Address, port)
	} else {
		one, errPort := netx.HostPort(target.Address, port)
		if errPort != nil {
			err = errPort
		} else {
			addrs = []string{one}
		}
	}
	if err != nil {
		return Sample{HitRate: -1}, models.Status{Error: err.Error()}
	}
	var last error
	for _, addr := range addrs {
		sample, status, err := infoAt(ctx, addr, auth, target.UserName, target.Secret)
		if err == nil {
			return sample, status
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("Redis 没有可用地址")
	}
	return Sample{HitRate: -1}, models.Status{Error: last.Error()}
}

func collectCluster(ctx context.Context, target conn.Open, auth string) (Sample, models.Status) {
	addrs, err := netx.HostPorts(target.Address, 6379)
	if err != nil {
		return Sample{HitRate: -1}, models.Status{Error: err.Error()}
	}
	var last error
	for _, addr := range addrs {
		raw, err := dial(ctx, addr)
		if err != nil {
			last = err
			continue
		}
		sample, status, err := clusterOn(raw, auth, target.UserName, target.Secret)
		_ = raw.Close()
		if err == nil {
			return sample, status
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("Redis 集群没有可用节点")
	}
	return Sample{HitRate: -1}, models.Status{Error: last.Error()}
}

func clusterOn(raw net.Conn, auth, user, secret string) (Sample, models.Status, error) {
	if err := authenticate(raw, auth, user, secret); err != nil {
		return Sample{HitRate: -1}, models.Status{}, err
	}
	body, err := command(raw, "CLUSTER", "INFO")
	if err != nil {
		return Sample{HitRate: -1}, models.Status{}, err
	}
	state, nodes, failSlots, size := ParseCluster(body)
	info, err := command(raw, "INFO")
	sample := Sample{HitRate: -1, KnownNodes: nodes, FailSlots: failSlots, ConnectedSlaves: size, Role: state}
	if err == nil {
		parsed := ParseInfo(info)
		copyUsage(&sample, parsed)
		readSlow(raw, &sample)
	}
	status := models.Status{
		Up: state == "ok", Version: sample.Version, Role: state,
		Summary: fmt.Sprintf("集群 %s · 节点 %d · 失败槽 %d", empty(state, "未知"), nodes, failSlots),
	}
	if state != "ok" {
		status.Error = "Redis 集群不可用"
	}
	return sample, status, nil
}

func collectSentinel(ctx context.Context, target conn.Open, auth string) (Sample, models.Status) {
	master := strings.TrimSpace(target.DatabaseName)
	if master == "" {
		return Sample{HitRate: -1}, models.Status{Error: "请填写主节点名"}
	}
	addrs, err := netx.HostPorts(target.Address, 26379)
	if err != nil {
		return Sample{HitRate: -1}, models.Status{Error: err.Error()}
	}
	var last error
	for _, addr := range addrs {
		sample, status, err := sentinelAt(ctx, addr, master, auth, target.UserName, target.Secret)
		if err == nil {
			return sample, status
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("没有可用的哨兵")
	}
	return Sample{HitRate: -1}, models.Status{Error: last.Error()}
}

func sentinelAt(ctx context.Context, addr, master, auth, user, secret string) (Sample, models.Status, error) {
	raw, err := dial(ctx, addr)
	if err != nil {
		return Sample{}, models.Status{}, err
	}
	defer raw.Close()
	if err := authenticate(raw, auth, user, secret); err != nil {
		return Sample{}, models.Status{}, err
	}
	fields, err := commandList(raw, "SENTINEL", "MASTER", master)
	if err != nil {
		return Sample{}, models.Status{}, err
	}
	view := ParseSentinel(fields)
	sample := Sample{
		HitRate: -1, Role: "sentinel", ConnectedSlaves: view.Slaves, KnownNodes: view.Sentinels,
	}
	if view.Down {
		sample.FailSlots = 1
	}
	status := models.Status{
		Up: !view.Down, Role: "sentinel",
		Summary: fmt.Sprintf("主节点 %s · 从节点 %d · 哨兵 %d", master, view.Slaves, view.Sentinels),
	}
	if view.Down {
		status.Error = "哨兵看到主节点已下线"
		return sample, status, nil
	}
	if view.IP != "" {
		masterAddr := net.JoinHostPort(view.IP, empty(view.Port, "6379"))
		if node, nodeStatus, err := infoAt(ctx, masterAddr, auth, user, secret); err == nil {
			copyUsage(&sample, node)
			sample.ConnectedSlaves = node.ConnectedSlaves
			status.Version = node.Version
			status.Summary = fmt.Sprintf("主节点 %s · 角色 %s · 从节点 %d · 哨兵 %d", master, empty(node.Role, "master"), node.ConnectedSlaves, view.Sentinels)
			_ = nodeStatus
		}
	}
	return sample, status, nil
}

func infoAt(ctx context.Context, addr, auth, user, secret string) (Sample, models.Status, error) {
	raw, err := dial(ctx, addr)
	if err != nil {
		return Sample{HitRate: -1}, models.Status{}, err
	}
	defer raw.Close()
	if err := authenticate(raw, auth, user, secret); err != nil {
		return Sample{HitRate: -1}, models.Status{}, err
	}
	body, err := command(raw, "INFO")
	if err != nil {
		return Sample{HitRate: -1}, models.Status{}, err
	}
	sample := ParseInfo(body)
	readSlow(raw, &sample)
	return sample, models.Status{
		Up: true, Version: sample.Version, Role: sample.Role,
		Summary: fmt.Sprintf("角色 %s · 连接 %d · 内存 %dMB", empty(sample.Role, "未知"), sample.ConnectedClients, sample.UsedMemoryBytes/1024/1024),
	}, nil
}

func dial(ctx context.Context, addr string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	return raw, nil
}

func authenticate(raw net.Conn, auth, user, secret string) error {
	switch auth {
	case "acl":
		if user == "" || secret == "" {
			return fmt.Errorf("ACL 需要用户名和口令")
		}
		_, err := command(raw, "AUTH", user, secret)
		return err
	case "password":
		if secret == "" {
			return fmt.Errorf("请填写 Redis 口令")
		}
		_, err := command(raw, "AUTH", secret)
		return err
	default:
		return nil
	}
}

// SentinelView 是哨兵对主节点的判断。
type SentinelView struct {
	IP        string
	Port      string
	Down      bool
	Slaves    int64
	Sentinels int64
}

// ParseSentinel 解析 SENTINEL MASTER 的键值列表。
func ParseSentinel(fields []string) SentinelView {
	pairs := map[string]string{}
	for i := 0; i+1 < len(fields); i += 2 {
		pairs[fields[i]] = fields[i+1]
	}
	flags := pairs["flags"]
	return SentinelView{
		IP: pairs["ip"], Port: pairs["port"],
		Down:   strings.Contains(flags, "s_down") || strings.Contains(flags, "o_down"),
		Slaves: atoi(pairs["num-slaves"]), Sentinels: atoi(pairs["num-sentinels"]),
	}
}

// ParseCluster 解析 CLUSTER INFO。
func ParseCluster(text string) (state string, nodes, failSlots, size int64) {
	fields := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok {
			fields[key] = val
		}
	}
	return fields["cluster_state"], atoi(fields["cluster_known_nodes"]), atoi(fields["cluster_slots_fail"]), atoi(fields["cluster_size"])
}

// ParseInfo 解析 INFO 文本。没有命中统计时命中率为 -1。
func ParseInfo(text string) Sample {
	fields := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		fields[key] = val
	}
	sample := Sample{
		UsedMemoryBytes:     atoi(fields["used_memory"]),
		MaxMemoryBytes:      atoi(fields["maxmemory"]),
		UsedMemoryPeakBytes: atoi(fields["used_memory_peak"]),
		ConnectedClients:    atoi(fields["connected_clients"]),
		BlockedClients:      atoi(fields["blocked_clients"]),
		OpsPerSec:           atof(fields["instantaneous_ops_per_sec"]),
		NetInKbps:           atof(fields["instantaneous_input_kbps"]),
		NetOutKbps:          atof(fields["instantaneous_output_kbps"]),
		ConnectedSlaves:     atoi(fields["connected_slaves"]),
		UptimeSec:           atoi(fields["uptime_in_seconds"]),
		Version:             fields["redis_version"],
		Role:                fields["role"],
		HitRate:             -1,
		MemUsagePercent:     -1,
		MemFragmentation:    -1,
		SlowlogLen:          -1,
		ReplicaLagSec:       -1,
		MasterLinkDown:      -1,
		RdbOk:               -1,
		AofOk:               -1,
		EvictedPerSec:       -1,
		ExpiredPerSec:       -1,
		RejectedPerSec:      -1,
		KeyspaceHits:        atoi(fields["keyspace_hits"]),
		KeyspaceMisses:      atoi(fields["keyspace_misses"]),
		EvictedKeys:         atoi(fields["evicted_keys"]),
		ExpiredKeys:         atoi(fields["expired_keys"]),
		RejectedConnections: atoi(fields["rejected_connections"]),
	}
	if sample.MaxMemoryBytes > 0 {
		sample.MemUsagePercent = float64(sample.UsedMemoryBytes) / float64(sample.MaxMemoryBytes) * 100
	}
	if _, ok := fields["mem_fragmentation_ratio"]; ok {
		sample.MemFragmentation = atof(fields["mem_fragmentation_ratio"])
	}
	sample.ReplicaLagSec, sample.MasterLinkDown = replicationOf(fields, sample.Role)
	sample.RdbOk = statusFlag(fields["rdb_last_bgsave_status"])
	if fields["aof_enabled"] == "1" {
		sample.AofOk = statusFlag(fields["aof_last_write_status"])
	}
	for key, val := range fields {
		if _, ok := dbIndex(key); !ok {
			continue
		}
		item := DbStat{Name: key, Keys: fieldNum(val, "keys"), Expires: fieldNum(val, "expires")}
		sample.DBs = append(sample.DBs, item)
		sample.KeyCount += item.Keys
	}
	sort.Slice(sample.DBs, func(i, j int) bool {
		left, _ := dbIndex(sample.DBs[i].Name)
		right, _ := dbIndex(sample.DBs[j].Name)
		return left < right
	})
	if sample.KeyspaceHits+sample.KeyspaceMisses > 0 {
		sample.HitRate = float64(sample.KeyspaceHits) / float64(sample.KeyspaceHits+sample.KeyspaceMisses) * 100
	}
	return sample
}

// prior 是上一采集点的累计计数，用来把命中率和淘汰换成区间值。
type prior struct {
	Hits, Misses, Evicted, Expired, Rejected, Uptime int64
	At                                               time.Time
}

// applyRates 用相邻两点的增量覆盖命中率。没有上一拍、实例重启或区间内没有键查找时，命中率保持 -1。
func applyRates(sample *Sample, prev *prior, now time.Time) {
	sample.HitRate = -1
	sample.EvictedPerSec = -1
	sample.ExpiredPerSec = -1
	sample.RejectedPerSec = -1
	if prev == nil || sample.UptimeSec < prev.Uptime {
		return
	}
	sec := now.Sub(prev.At).Seconds()
	if sec < 1 || sec > 3600 {
		return
	}
	dh := sample.KeyspaceHits - prev.Hits
	dm := sample.KeyspaceMisses - prev.Misses
	if dh < 0 || dm < 0 {
		return
	}
	if dh+dm > 0 {
		sample.HitRate = float64(dh) / float64(dh+dm) * 100
	}
	sample.EvictedPerSec = perSec(sample.EvictedKeys, prev.Evicted, sec)
	sample.ExpiredPerSec = perSec(sample.ExpiredKeys, prev.Expired, sec)
	sample.RejectedPerSec = perSec(sample.RejectedConnections, prev.Rejected, sec)
}

func perSec(cur, prev int64, sec float64) float64 {
	if cur < prev {
		return -1
	}
	return float64(cur-prev) / sec
}

func replicationOf(fields map[string]string, role string) (lag, link float64) {
	if role == "slave" || role == "replica" {
		switch fields["master_link_status"] {
		case "up":
			return atof(fields["master_last_io_seconds_ago"]), 0
		case "down":
			return -1, 1
		default:
			return -1, -1
		}
	}
	lag = -1
	for key, val := range fields {
		if !slaveLine(key) || !strings.Contains(val, "lag=") {
			continue
		}
		item := float64(fieldNum(val, "lag"))
		if item > lag {
			lag = item
		}
	}
	return lag, -1
}

func slaveLine(key string) bool {
	if !strings.HasPrefix(key, "slave") {
		return false
	}
	_, err := strconv.Atoi(key[len("slave"):])
	return err == nil
}

func statusFlag(value string) float64 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ok":
		return 1
	case "err", "error":
		return 0
	default:
		return -1
	}
}

func clearDerived(sample *Sample) {
	sample.HitRate = -1
	sample.MemUsagePercent = -1
	sample.MemFragmentation = -1
	sample.SlowlogLen = -1
	sample.ReplicaLagSec = -1
	sample.MasterLinkDown = -1
	sample.RdbOk = -1
	sample.AofOk = -1
	sample.EvictedPerSec = -1
	sample.ExpiredPerSec = -1
	sample.RejectedPerSec = -1
}

func copyUsage(dst *Sample, src Sample) {
	dst.UsedMemoryBytes = src.UsedMemoryBytes
	dst.MaxMemoryBytes = src.MaxMemoryBytes
	dst.MemUsagePercent = src.MemUsagePercent
	dst.UsedMemoryPeakBytes = src.UsedMemoryPeakBytes
	dst.ConnectedClients = src.ConnectedClients
	dst.BlockedClients = src.BlockedClients
	dst.OpsPerSec = src.OpsPerSec
	dst.HitRate = src.HitRate
	dst.NetInKbps = src.NetInKbps
	dst.NetOutKbps = src.NetOutKbps
	dst.UptimeSec = src.UptimeSec
	dst.Version = src.Version
	dst.MemFragmentation = src.MemFragmentation
	dst.KeyCount = src.KeyCount
	dst.SlowlogLen = src.SlowlogLen
	dst.ReplicaLagSec = src.ReplicaLagSec
	dst.MasterLinkDown = src.MasterLinkDown
	dst.RdbOk = src.RdbOk
	dst.AofOk = src.AofOk
	dst.EvictedPerSec = src.EvictedPerSec
	dst.ExpiredPerSec = src.ExpiredPerSec
	dst.RejectedPerSec = src.RejectedPerSec
	dst.KeyspaceHits = src.KeyspaceHits
	dst.KeyspaceMisses = src.KeyspaceMisses
	dst.EvictedKeys = src.EvictedKeys
	dst.ExpiredKeys = src.ExpiredKeys
	dst.RejectedConnections = src.RejectedConnections
	dst.DBs = src.DBs
	dst.Slow = src.Slow
}

func readSlow(raw net.Conn, sample *Sample) {
	if n, err := command(raw, "SLOWLOG", "LEN"); err == nil {
		sample.SlowlogLen = atoi(n)
	}
	value, err := commandAny(raw, "SLOWLOG", "GET", "8")
	if err != nil {
		return
	}
	sample.Slow = ParseSlowlog(value)
}

func dbIndex(key string) (int, bool) {
	if !strings.HasPrefix(key, "db") {
		return 0, false
	}
	n, err := strconv.Atoi(key[2:])
	return n, err == nil
}

func fieldNum(text, name string) int64 {
	for _, part := range strings.Split(text, ",") {
		key, val, ok := strings.Cut(part, "=")
		if ok && key == name {
			return atoi(val)
		}
	}
	return 0
}

// ParseSlowlog 解析 SLOWLOG GET 的嵌套数组。
func ParseSlowlog(value any) []SlowOp {
	rows, ok := value.([]any)
	if !ok {
		return nil
	}
	var out []SlowOp
	for _, row := range rows {
		fields, ok := row.([]any)
		if !ok || len(fields) < 4 {
			continue
		}
		args, _ := fields[3].([]any)
		parts := make([]string, 0, len(args))
		for _, arg := range args {
			if text, ok := arg.(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		out = append(out, SlowOp{Usec: atoi(fmt.Sprint(fields[2])), Command: cutRunes(strings.Join(parts, " "), 120)})
	}
	return out
}

func cutRunes(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n])
}

func command(conn net.Conn, args ...string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	if _, err := io.WriteString(conn, b.String()); err != nil {
		return "", err
	}
	return readRESP(bufio.NewReader(conn))
}

func commandList(conn net.Conn, args ...string) ([]string, error) {
	value, err := commandAny(conn, args...)
	if err != nil {
		return nil, err
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("Redis 应答无效")
	}
	out := make([]string, len(list))
	for i, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("Redis 应答无效")
		}
		out[i] = text
	}
	return out, nil
}

func commandAny(conn net.Conn, args ...string) (any, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	if _, err := io.WriteString(conn, b.String()); err != nil {
		return nil, err
	}
	return readValue(bufio.NewReader(conn))
}

func readRESP(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", fmt.Errorf("Redis 应答为空")
	}
	switch line[0] {
	case '+', ':':
		return line[1:], nil
	case '-':
		return "", fmt.Errorf("%s", line[1:])
	case '$':
		n, err := strconv.Atoi(line[1:])
		if err != nil || n < 0 {
			return "", fmt.Errorf("Redis 应答无效")
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	default:
		return "", fmt.Errorf("Redis 应答无效")
	}
}

func readValue(r *bufio.Reader) (any, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return nil, fmt.Errorf("Redis 应答为空")
	}
	switch line[0] {
	case '+', ':':
		return line[1:], nil
	case '-':
		return nil, fmt.Errorf("%s", line[1:])
	case '$':
		n, err := strconv.Atoi(line[1:])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("Redis 应答无效")
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	case '*':
		n, err := strconv.Atoi(line[1:])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("Redis 应答无效")
		}
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			item, err := readValue(r)
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("Redis 应答无效")
	}
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	now := time.Now().UTC()
	if status.Up {
		prev, err := lastCounters(ctx, db, tenantID, componentID)
		if err != nil {
			return err
		}
		applyRates(&sample, prev, now)
	} else {
		clearDerived(&sample)
	}
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(now),
		"usedMemoryBytes": sample.UsedMemoryBytes, "connectedClients": sample.ConnectedClients,
		"opsPerSec": sample.OpsPerSec, "hitRate": sample.HitRate,
		"connectedSlaves": sample.ConnectedSlaves, "knownNodes": sample.KnownNodes,
		"failSlots": sample.FailSlots, "uptimeSec": sample.UptimeSec,
		"memFragmentation": sample.MemFragmentation, "keyCount": sample.KeyCount, "slowlogLen": sample.SlowlogLen,
		"maxMemoryBytes": sample.MaxMemoryBytes, "memUsagePercent": sample.MemUsagePercent,
		"usedMemoryPeakBytes": sample.UsedMemoryPeakBytes, "blockedClients": sample.BlockedClients,
		"netInKbps": sample.NetInKbps, "netOutKbps": sample.NetOutKbps,
		"replicaLagSec": sample.ReplicaLagSec, "masterLinkDown": sample.MasterLinkDown,
		"rdbOk": sample.RdbOk, "aofOk": sample.AofOk,
		"evictedPerSec": sample.EvictedPerSec, "expiredPerSec": sample.ExpiredPerSec, "rejectedPerSec": sample.RejectedPerSec,
		"keyspaceHits": sample.KeyspaceHits, "keyspaceMisses": sample.KeyspaceMisses,
		"evictedKeys": sample.EvictedKeys, "expiredKeys": sample.ExpiredKeys, "rejectedConnections": sample.RejectedConnections,
		"extProperty": redisDetail(sample),
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_REDIS_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  usedMemoryBytes, connectedClients, opsPerSec, hitRate, connectedSlaves, knownNodes, failSlots, uptimeSec,
		  memFragmentation, keyCount, slowlogLen,
		  maxMemoryBytes, memUsagePercent, usedMemoryPeakBytes, blockedClients, netInKbps, netOutKbps,
		  replicaLagSec, masterLinkDown, rdbOk, aofOk, evictedPerSec, expiredPerSec, rejectedPerSec,
		  keyspaceHits, keyspaceMisses, evictedKeys, expiredKeys, rejectedConnections, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @usedMemoryBytes, @connectedClients, @opsPerSec, @hitRate, @connectedSlaves, @knownNodes, @failSlots, @uptimeSec,
		  @memFragmentation, @keyCount, @slowlogLen,
		  @maxMemoryBytes, @memUsagePercent, @usedMemoryPeakBytes, @blockedClients, @netInKbps, @netOutKbps,
		  @replicaLagSec, @masterLinkDown, @rdbOk, @aofOk, @evictedPerSec, @expiredPerSec, @rejectedPerSec,
		  @keyspaceHits, @keyspaceMisses, @evictedKeys, @expiredKeys, @rejectedConnections, @extProperty,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	return err
}

func lastCounters(ctx context.Context, db *dbx.DB, tenantID, componentID string) (*prior, error) {
	var hits, misses, evicted, expired, rejected, uptime, observed any
	err := store.Scan(ctx, db, `
		SELECT keyspaceHits, keyspaceMisses, evictedKeys, expiredKeys, rejectedConnections, uptimeSec, observedTime
		FROM FG_REDIS_METRIC
		WHERE tenantId = @tenantId AND componentId = @componentId
		ORDER BY observedTime DESC
		LIMIT 1`, dbx.Args{"tenantId": tenantID, "componentId": componentID},
		&hits, &misses, &evicted, &expired, &rejected, &uptime, &observed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	at, ok := store.TimeValue(observed)
	if !ok {
		return nil, nil
	}
	return &prior{
		Hits: int64(row.Float(hits)), Misses: int64(row.Float(misses)),
		Evicted: int64(row.Float(evicted)), Expired: int64(row.Float(expired)),
		Rejected: int64(row.Float(rejected)), Uptime: int64(row.Float(uptime)), At: at,
	}, nil
}

func redisDetail(sample Sample) string {
	if len(sample.DBs) == 0 && len(sample.Slow) == 0 {
		return ""
	}
	body, err := json.Marshal(struct {
		DBs  []DbStat `json:"dbs,omitempty"`
		Slow []SlowOp `json:"slow,omitempty"`
	}{DBs: sample.DBs, Slow: sample.Slow})
	if err != nil {
		return ""
	}
	return string(body)
}

func atoi(v string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n
}

func atof(v string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return n
}

func empty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
