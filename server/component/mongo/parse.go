package mongo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"fluxguard/server/component/row"
)

const (
	detailMemberCap  = 64
	detailDbCap      = 128
	detailRunningCap = 128
)

// Member 是副本集里的一个成员，写在采集记录的 extProperty 里。
type Member struct {
	Name   string  `json:"name"`
	State  string  `json:"state"`
	Health float64 `json:"health"`
	Lag    float64 `json:"lag"`
	Self   bool    `json:"self,omitempty"`
}

// Database 是一个库的磁盘占用。
type Database struct {
	Name  string  `json:"name"`
	Bytes float64 `json:"bytes"`
	Empty bool    `json:"empty,omitempty"`
}

// Running 是一条还在执行、并且已经超过 1 秒的操作。
type Running struct {
	Op     string  `json:"op"`
	NS     string  `json:"ns"`
	Secs   float64 `json:"secs"`
	Client string  `json:"client,omitempty"`
	OpID   int64   `json:"opid,omitempty"`
}

// Sample 是一轮 MongoDB 采集。未知的数值保持 -1，避免把没读到写成 0。
type Sample struct {
	ConnectionsCurrent   int64
	ConnectionsAvailable int64
	ConnectionsActive    int64
	ConnUsagePercent     float64
	ResidentBytes        int64
	CacheUsedBytes       int64
	CacheMaxBytes        int64
	CacheDirtyBytes      int64
	CacheUsagePercent    float64
	CacheDirtyPercent    float64
	OpsPerSec            float64
	OpInsertPerSec       float64
	OpQueryPerSec        float64
	OpUpdatePerSec       float64
	OpDeletePerSec       float64
	OpCommandPerSec      float64
	OpGetmorePerSec      float64
	OpInsert             int64
	OpQuery              int64
	OpUpdate             int64
	OpDelete             int64
	OpGetmore            int64
	OpCommand            int64
	HasOps               bool
	ReplicaLagSec        float64
	MemberCount          int64
	UnhealthyMembers     int64
	NetInKbps            float64
	NetOutKbps           float64
	NetIn                int64
	NetOut               int64
	HasNet               bool
	QueuedReaders        int64
	QueuedWriters        int64
	ReadTickets          int64
	WriteTickets         int64
	AssertPerSec         float64
	Asserts              int64
	HasAsserts           bool
	SlowOps              int64
	UptimeSec            int64
	DataBytes            int64
	Version              string
	Role                 string
	SetName              string
	Members              []Member
	Databases            []Database
	Running              []Running
	MemberNote           string
	DbNote               string
	RunningNote          string
}

// prior 是上一采集点的累计计数，用来把操作、流量和断言换成区间速率。
type prior struct {
	Insert, Query, Update, Delete, Getmore, Command int64
	NetIn, NetOut, Asserts, Uptime                  int64
	At                                              time.Time
}

func newSample() Sample {
	return Sample{
		ConnectionsCurrent: -1, ConnectionsAvailable: -1, ConnectionsActive: -1, ConnUsagePercent: -1,
		ResidentBytes: -1, CacheUsedBytes: -1, CacheMaxBytes: -1, CacheDirtyBytes: -1,
		CacheUsagePercent: -1, CacheDirtyPercent: -1,
		OpsPerSec: -1, OpInsertPerSec: -1, OpQueryPerSec: -1, OpUpdatePerSec: -1,
		OpDeletePerSec: -1, OpCommandPerSec: -1, OpGetmorePerSec: -1,
		OpInsert: -1, OpQuery: -1, OpUpdate: -1, OpDelete: -1, OpGetmore: -1, OpCommand: -1,
		ReplicaLagSec: -1, MemberCount: -1, UnhealthyMembers: -1,
		NetInKbps: -1, NetOutKbps: -1, NetIn: -1, NetOut: -1,
		QueuedReaders: -1, QueuedWriters: -1, ReadTickets: -1, WriteTickets: -1,
		AssertPerSec: -1, Asserts: -1, SlowOps: -1, UptimeSec: -1, DataBytes: -1,
	}
}

func sampleFrom(doc map[string]any) Sample {
	sample := newSample()
	sample.Version = text(dig(doc, "version"))
	sample.Role = roleOf(doc)
	sample.SetName = text(dig(doc, "repl", "setName"))
	if v, ok := lookup(doc, "uptime"); ok {
		sample.UptimeSec = int64(num(v))
	}
	if v, ok := lookup(doc, "connections", "current"); ok {
		sample.ConnectionsCurrent = int64(num(v))
	}
	if v, ok := lookup(doc, "connections", "available"); ok {
		sample.ConnectionsAvailable = int64(num(v))
	}
	if v, ok := lookup(doc, "connections", "active"); ok {
		sample.ConnectionsActive = int64(num(v))
	}
	if sample.ConnectionsCurrent >= 0 && sample.ConnectionsAvailable >= 0 {
		total := sample.ConnectionsCurrent + sample.ConnectionsAvailable
		if total > 0 {
			sample.ConnUsagePercent = float64(sample.ConnectionsCurrent) / float64(total) * 100
		}
	}
	if v, ok := lookup(doc, "mem", "resident"); ok {
		sample.ResidentBytes = int64(num(v) * 1024 * 1024)
	}
	if _, ok := lookup(doc, "wiredTiger", "cache"); ok {
		if v, ok := lookup(doc, "wiredTiger", "cache", "bytes currently in the cache"); ok {
			sample.CacheUsedBytes = int64(num(v))
		}
		if v, ok := lookup(doc, "wiredTiger", "cache", "maximum bytes configured"); ok {
			sample.CacheMaxBytes = int64(num(v))
		}
		if v, ok := lookup(doc, "wiredTiger", "cache", "tracked dirty bytes in the cache"); ok {
			sample.CacheDirtyBytes = int64(num(v))
		}
		if sample.CacheMaxBytes > 0 {
			if sample.CacheUsedBytes >= 0 {
				sample.CacheUsagePercent = float64(sample.CacheUsedBytes) / float64(sample.CacheMaxBytes) * 100
			}
			if sample.CacheDirtyBytes >= 0 {
				sample.CacheDirtyPercent = float64(sample.CacheDirtyBytes) / float64(sample.CacheMaxBytes) * 100
			}
		}
	}
	if v, ok := lookup(doc, "wiredTiger", "concurrentTransactions", "read", "available"); ok {
		sample.ReadTickets = int64(num(v))
	}
	if v, ok := lookup(doc, "wiredTiger", "concurrentTransactions", "write", "available"); ok {
		sample.WriteTickets = int64(num(v))
	}
	if v, ok := lookup(doc, "globalLock", "currentQueue", "readers"); ok {
		sample.QueuedReaders = int64(num(v))
	}
	if v, ok := lookup(doc, "globalLock", "currentQueue", "writers"); ok {
		sample.QueuedWriters = int64(num(v))
	}
	if _, ok := lookup(doc, "opcounters"); ok {
		sample.HasOps = true
		sample.OpInsert = counter(doc, "opcounters", "insert")
		sample.OpQuery = counter(doc, "opcounters", "query")
		sample.OpUpdate = counter(doc, "opcounters", "update")
		sample.OpDelete = counter(doc, "opcounters", "delete")
		sample.OpGetmore = counter(doc, "opcounters", "getmore")
		sample.OpCommand = counter(doc, "opcounters", "command")
	}
	if _, ok := lookup(doc, "network"); ok {
		sample.HasNet = true
		sample.NetIn = counter(doc, "network", "bytesIn")
		sample.NetOut = counter(doc, "network", "bytesOut")
	}
	if _, ok := lookup(doc, "asserts"); ok {
		sample.HasAsserts = true
		sample.Asserts = counter(doc, "asserts", "regular") + counter(doc, "asserts", "warning") +
			counter(doc, "asserts", "msg") + counter(doc, "asserts", "user")
	}
	return sample
}

func counter(doc map[string]any, path ...string) int64 {
	v, ok := lookup(doc, path...)
	if !ok {
		return 0
	}
	return int64(num(v))
}

func roleOf(doc map[string]any) string {
	if text(dig(doc, "process")) == "mongos" {
		return "mongos"
	}
	if truthy(dig(doc, "repl", "arbiterOnly")) || truthy(dig(doc, "repl", "isarbiter")) {
		return "arbiter"
	}
	if truthy(dig(doc, "repl", "isWritablePrimary")) || truthy(dig(doc, "repl", "ismaster")) {
		return "primary"
	}
	if truthy(dig(doc, "repl", "secondary")) {
		return "secondary"
	}
	if text(dig(doc, "repl", "setName")) != "" {
		return "unknown"
	}
	return "standalone"
}

func readReplica(sample *Sample, doc map[string]any, me string) {
	if num(dig(doc, "ok")) < 1 {
		sample.MemberNote = "不是副本集，或暂时读不到成员。"
		return
	}
	if name := text(dig(doc, "set")); name != "" {
		sample.SetName = name
	}
	type seen struct {
		member Member
		opt    float64
	}
	var rows []seen
	var unhealthy int64
	var primarySec float64
	var hasPrimary bool
	for _, item := range asSlice(dig(doc, "members")) {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := text(dig(raw, "name"))
		if name == "" {
			name = "—"
		}
		state := text(dig(raw, "stateStr"))
		if state == "" {
			state = "UNKNOWN"
		}
		health := -1.0
		if v, ok := lookup(raw, "health"); ok {
			if num(v) > 0 {
				health = 1
			} else {
				health = 0
				unhealthy++
			}
		}
		opt := optSeconds(raw)
		self := truthy(dig(raw, "self")) || (me != "" && name == me)
		if state == "PRIMARY" && opt >= 0 {
			primarySec = opt
			hasPrimary = true
		}
		rows = append(rows, seen{member: Member{
			Name: row.Cut(name, 120), State: row.Cut(state, 32), Health: health, Lag: -1, Self: self,
		}, opt: opt})
		if self && sample.Role != "mongos" {
			switch state {
			case "PRIMARY":
				sample.Role = "primary"
			case "SECONDARY":
				sample.Role = "secondary"
			case "ARBITER":
				sample.Role = "arbiter"
			}
		}
	}
	sample.MemberCount = int64(len(rows))
	sample.UnhealthyMembers = unhealthy
	var selfState string
	var selfLag = -1.0
	var maxSecondary = -1.0
	var secondaries int
	members := make([]Member, 0, len(rows))
	for _, item := range rows {
		lag := -1.0
		if item.member.State == "PRIMARY" {
			lag = 0
		} else if hasPrimary && item.opt >= 0 {
			lag = primarySec - item.opt
			if lag < 0 {
				lag = 0
			}
		}
		item.member.Lag = lag
		if item.member.Self {
			selfState = item.member.State
			selfLag = lag
		}
		if item.member.State == "SECONDARY" {
			secondaries++
			if lag > maxSecondary {
				maxSecondary = lag
			}
		}
		members = append(members, item.member)
	}
	if selfState == "" && sample.Role == "primary" {
		selfState = "PRIMARY"
	}
	switch selfState {
	case "PRIMARY":
		if maxSecondary >= 0 {
			sample.ReplicaLagSec = maxSecondary
		} else if secondaries == 0 {
			sample.ReplicaLagSec = 0
		}
	case "SECONDARY":
		sample.ReplicaLagSec = selfLag
	}
	if len(members) > detailMemberCap {
		sample.MemberNote = fmt.Sprintf("成员共 %d 个，这里列出前 %d 个。", len(members), detailMemberCap)
		members = members[:detailMemberCap]
	}
	sample.Members = members
}

func optSeconds(raw map[string]any) float64 {
	if raw == nil {
		return -1
	}
	if v, ok := lookup(raw, "optimeDate"); ok {
		ms := num(v)
		if ms > 1e11 {
			return ms / 1000
		}
		if ms > 1e9 {
			return ms
		}
	}
	if v, ok := lookup(raw, "optime", "ts"); ok {
		sec := num(v)
		if sec > 1e9 {
			return sec
		}
	}
	if v, ok := lookup(raw, "optime"); ok {
		sec := num(v)
		if sec > 1e9 {
			return sec
		}
	}
	return -1
}

func readDatabases(sample *Sample, doc map[string]any) {
	if num(dig(doc, "ok")) < 1 {
		sample.DbNote = "读不到库列表。"
		return
	}
	var out []Database
	for _, item := range asSlice(dig(doc, "databases")) {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := text(dig(raw, "name"))
		if name == "" {
			continue
		}
		out = append(out, Database{
			Name: row.Cut(name, 80), Bytes: num(dig(raw, "sizeOnDisk")), Empty: truthy(dig(raw, "empty")),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes == out[j].Bytes {
			return out[i].Name < out[j].Name
		}
		return out[i].Bytes > out[j].Bytes
	})
	if total, ok := lookup(doc, "totalSize"); ok {
		sample.DataBytes = int64(num(total))
	} else {
		var sum float64
		for _, item := range out {
			if item.Bytes > 0 {
				sum += item.Bytes
			}
		}
		sample.DataBytes = int64(sum)
	}
	if len(out) > detailDbCap {
		sample.DbNote = fmt.Sprintf("库共 %d 个，这里列出占用最高的前 %d 个。", len(out), detailDbCap)
		out = out[:detailDbCap]
	}
	sample.Databases = out
}

func readRunning(sample *Sample, doc map[string]any) {
	if num(dig(doc, "ok")) < 1 {
		sample.RunningNote = "读不到正在执行的操作。"
		sample.SlowOps = -1
		return
	}
	var out []Running
	for _, item := range asSlice(dig(doc, "inprog")) {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		secs := num(dig(raw, "secs_running"))
		if secs < 1 {
			micro := num(dig(raw, "microsecs_running"))
			if micro >= 1_000_000 {
				secs = micro / 1e6
			}
		}
		if secs < 1 {
			continue
		}
		op := text(dig(raw, "op"))
		if op == "" {
			op = "—"
		}
		opid, _ := intOf(dig(raw, "opid"))
		out = append(out, Running{
			Op: row.Cut(op, 24), NS: row.Cut(text(dig(raw, "ns")), 80), Secs: secs,
			Client: row.Cut(text(dig(raw, "client")), 64), OpID: opid,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Secs > out[j].Secs })
	sample.SlowOps = int64(len(out))
	if len(out) > detailRunningCap {
		sample.RunningNote = fmt.Sprintf("正在执行的慢操作共 %d 条，这里列出最久的前 %d 条。", len(out), detailRunningCap)
		out = out[:detailRunningCap]
	}
	sample.Running = out
}

func asSlice(v any) []any {
	doc, ok := v.(map[string]any)
	if !ok || len(doc) == 0 {
		return nil
	}
	max := -1
	for key := range doc {
		n, err := atoi(key)
		if err != nil || n < 0 {
			continue
		}
		if n > max {
			max = n
		}
	}
	if max < 0 {
		return nil
	}
	out := make([]any, max+1)
	for key, item := range doc {
		n, err := atoi(key)
		if err != nil || n < 0 || n > max {
			continue
		}
		out[n] = item
	}
	return out
}

func atoi(text string) (int, error) {
	n := 0
	if text == "" {
		return 0, fmt.Errorf("invalid")
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// applyRates 用相邻两点的增量覆盖速率。没有上一拍、实例重启，或上一拍没记过这项计数时，速率保持 -1。
func applyRates(sample *Sample, prev *prior, now time.Time) {
	sample.OpInsertPerSec = -1
	sample.OpQueryPerSec = -1
	sample.OpUpdatePerSec = -1
	sample.OpDeletePerSec = -1
	sample.OpCommandPerSec = -1
	sample.OpGetmorePerSec = -1
	sample.OpsPerSec = -1
	sample.NetInKbps = -1
	sample.NetOutKbps = -1
	sample.AssertPerSec = -1
	if prev == nil {
		return
	}
	if prev.Uptime >= 0 && sample.UptimeSec >= 0 && sample.UptimeSec < prev.Uptime {
		return
	}
	sec := now.Sub(prev.At).Seconds()
	if sec < 1 || sec > 3600 {
		return
	}
	if sample.HasOps {
		sample.OpInsertPerSec = perSec(sample.OpInsert, prev.Insert, sec)
		sample.OpQueryPerSec = perSec(sample.OpQuery, prev.Query, sec)
		sample.OpUpdatePerSec = perSec(sample.OpUpdate, prev.Update, sec)
		sample.OpDeletePerSec = perSec(sample.OpDelete, prev.Delete, sec)
		sample.OpCommandPerSec = perSec(sample.OpCommand, prev.Command, sec)
		sample.OpGetmorePerSec = perSec(sample.OpGetmore, prev.Getmore, sec)
		sample.OpsPerSec = sumRates(
			sample.OpInsertPerSec, sample.OpQueryPerSec, sample.OpUpdatePerSec,
			sample.OpDeletePerSec, sample.OpCommandPerSec, sample.OpGetmorePerSec,
		)
	}
	if sample.HasNet {
		if sample.NetIn >= 0 && prev.NetIn >= 0 && sample.NetIn >= prev.NetIn {
			sample.NetInKbps = float64(sample.NetIn-prev.NetIn) / sec / 1024
		}
		if sample.NetOut >= 0 && prev.NetOut >= 0 && sample.NetOut >= prev.NetOut {
			sample.NetOutKbps = float64(sample.NetOut-prev.NetOut) / sec / 1024
		}
	}
	if sample.HasAsserts {
		sample.AssertPerSec = perSec(sample.Asserts, prev.Asserts, sec)
	}
}

func perSec(cur, prev int64, sec float64) float64 {
	if cur < 0 || prev < 0 || cur < prev || sec <= 0 {
		return -1
	}
	return float64(cur-prev) / sec
}

func sumRates(parts ...float64) float64 {
	seen := false
	var sum float64
	for _, part := range parts {
		if part < 0 {
			continue
		}
		seen = true
		sum += part
	}
	if !seen {
		return -1
	}
	return sum
}

func summaryOf(sample Sample) string {
	parts := []string{roleText(sample.Role)}
	if sample.ConnectionsCurrent >= 0 {
		parts = append(parts, fmt.Sprintf("连接 %d", sample.ConnectionsCurrent))
	}
	if sample.CacheUsedBytes >= 0 {
		parts = append(parts, "缓存 "+byteText(sample.CacheUsedBytes))
	}
	return strings.Join(parts, " · ")
}

func roleText(code string) string {
	switch code {
	case "primary":
		return "主节点"
	case "secondary":
		return "从节点"
	case "arbiter":
		return "仲裁"
	case "mongos":
		return "路由"
	case "standalone":
		return "单机"
	default:
		if code == "" {
			return "MongoDB"
		}
		return code
	}
}

func byteText(n int64) string {
	switch {
	case n >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(n)/1024/1024/1024)
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func mongoDetail(sample Sample) string {
	if sample.SetName == "" && len(sample.Members) == 0 && len(sample.Databases) == 0 && len(sample.Running) == 0 &&
		sample.MemberNote == "" && sample.DbNote == "" && sample.RunningNote == "" {
		return ""
	}
	body, err := json.Marshal(struct {
		SetName     string     `json:"setName,omitempty"`
		Members     []Member   `json:"members,omitempty"`
		Databases   []Database `json:"databases,omitempty"`
		Running     []Running  `json:"running,omitempty"`
		MemberNote  string     `json:"memberNote,omitempty"`
		DbNote      string     `json:"dbNote,omitempty"`
		RunningNote string     `json:"runningNote,omitempty"`
	}{
		SetName: sample.SetName, Members: sample.Members, Databases: sample.Databases, Running: sample.Running,
		MemberNote: sample.MemberNote, DbNote: sample.DbNote, RunningNote: sample.RunningNote,
	})
	if err != nil || string(body) == "{}" || string(body) == "null" {
		return ""
	}
	return string(body)
}
