// Package zk 用四字命令 mntr 采集，并写入 FG_COMPONENT_ZK_METRIC。
// 清单里的 zk 插件接上之后，采集和维护改由那个进程负责。
package zk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/pane"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	runtime.Register(catalog.ZK, probe{})
}

type probe struct{}

// Sample 是一轮 ZooKeeper 采集。
type Sample struct {
	AvgLatency          float64
	OutstandingRequests int64
	ZnodeCount          int64
	Followers           int64
	WatchCount          int64
	Version             string
	Role                string
	MinLatency          float64
	MaxLatency          float64
	Connections         int64
	Received            int64
	Sent                int64
	Zxid                string
	Source              string
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
	return series.Read(ctx, db, "FG_COMPONENT_ZK_METRIC", tenantID, componentID, catalog.Columns(catalog.ZK), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	addr, err := netx.HostPort(target.Address, 2181)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	mntrText, mntrErr := fourLetter(ctx, addr, "mntr")
	if mntrErr == nil && !mntrClosed(mntrText) {
		sample := ParseMntr(mntrText)
		if knownSample(sample) {
			sample.Source = "mntr"
			return sample, upStatus(sample)
		}
	}
	srvrText, srvrErr := fourLetter(ctx, addr, "srvr")
	if srvrErr != nil {
		if mntrErr != nil {
			return Sample{}, models.Status{Error: mntrErr.Error()}
		}
		if mntrClosed(mntrText) {
			return Sample{}, models.Status{Error: "ZooKeeper 未打开 mntr"}
		}
		return Sample{}, models.Status{Error: srvrErr.Error()}
	}
	sample := ParseSrvr(srvrText)
	if !knownSample(sample) {
		if mntrClosed(mntrText) {
			return Sample{}, models.Status{Error: "ZooKeeper 未打开 mntr"}
		}
		return sample, models.Status{Error: "mntr 应答无法识别"}
	}
	sample.Source = "srvr"
	return sample, upStatus(sample)
}

func fourLetter(ctx context.Context, addr, word string) (string, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", err
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	if _, err := io.WriteString(raw, word+"\n"); err != nil {
		return "", err
	}
	body, err := io.ReadAll(io.LimitReader(raw, 64*1024))
	if err != nil && len(body) == 0 {
		return "", err
	}
	return string(body), nil
}

func mntrClosed(text string) bool {
	text = strings.TrimSpace(text)
	return strings.Contains(text, "not executed") || strings.HasPrefix(text, "mntr is not")
}

func knownSample(sample Sample) bool {
	return sample.Role != "" || sample.ZnodeCount != 0 || sample.Version != ""
}

func upStatus(sample Sample) models.Status {
	return models.Status{
		Up: true, Version: sample.Version, Role: sample.Role,
		Summary: fmt.Sprintf("角色 %s · 节点 %d · 延迟 %.0fms", sample.Role, sample.ZnodeCount, sample.AvgLatency),
	}
}

// ParseMntr 解析 mntr 的键值行。
func ParseMntr(text string) Sample {
	fields := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields[strings.TrimPrefix(key, "zk_")] = val
	}
	return Sample{
		AvgLatency:          atof(fields["avg_latency"]),
		OutstandingRequests: atoi(fields["outstanding_requests"]),
		ZnodeCount:          atoi(fields["znode_count"]),
		Followers:           atoi(fields["followers"]),
		WatchCount:          atoi(fields["watch_count"]),
		Version:             fields["version"],
		Role:                fields["server_state"],
		MinLatency:          atof(fields["min_latency"]),
		MaxLatency:          atof(fields["max_latency"]),
		Connections:         atoi(fields["num_alive_connections"]),
		Received:            atoi(fields["packets_received"]),
		Sent:                atoi(fields["packets_sent"]),
		Zxid:                fields["zxid"],
	}
}

// ParseSrvr 解析默认白名单里的 srvr 正文。mntr 未开放时用它判断连通、角色和规模。
func ParseSrvr(text string) Sample {
	sample := Sample{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "zookeeper version":
			sample.Version = srvrVersion(val)
		case "latency min/avg/max":
			sample.MinLatency, sample.AvgLatency, sample.MaxLatency = latencyTriple(val)
		case "received":
			sample.Received = atoi(val)
		case "sent":
			sample.Sent = atoi(val)
		case "connections":
			sample.Connections = atoi(val)
		case "outstanding":
			sample.OutstandingRequests = atoi(val)
		case "zxid":
			sample.Zxid = val
		case "mode":
			sample.Role = strings.ToLower(val)
		case "node count":
			sample.ZnodeCount = atoi(val)
		}
	}
	return sample
}

func srvrVersion(val string) string {
	val = strings.TrimSpace(val)
	if i := strings.IndexByte(val, ','); i >= 0 {
		val = strings.TrimSpace(val[:i])
	}
	return val
}

func latencyTriple(val string) (min, avg, max float64) {
	parts := strings.Split(val, "/")
	if len(parts) != 3 {
		return 0, atof(val), 0
	}
	return atof(parts[0]), atof(parts[1]), atof(parts[2])
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(time.Now().UTC()),
		"avgLatency": sample.AvgLatency, "outstandingRequests": sample.OutstandingRequests,
		"znodeCount": sample.ZnodeCount, "followers": sample.Followers, "watchCount": sample.WatchCount,
		"extProperty": sampleDetail(sample),
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_COMPONENT_ZK_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  avgLatency, outstandingRequests, znodeCount, followers, watchCount, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @avgLatency, @outstandingRequests, @znodeCount, @followers, @watchCount, @extProperty,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	if err != nil {
		return err
	}
	return pane.Touch(ctx, db, catalog.ZK, args)
}

func sampleDetail(sample Sample) string {
	if sample.Source == "" && sample.Connections == 0 && sample.Received == 0 && sample.Zxid == "" {
		return ""
	}
	body, err := json.Marshal(struct {
		Source      string  `json:"source,omitempty"`
		MinLatency  float64 `json:"minLatency"`
		MaxLatency  float64 `json:"maxLatency"`
		Connections int64   `json:"connections"`
		Received    int64   `json:"received"`
		Sent        int64   `json:"sent"`
		Zxid        string  `json:"zxid,omitempty"`
	}{
		Source: sample.Source, MinLatency: sample.MinLatency, MaxLatency: sample.MaxLatency,
		Connections: sample.Connections, Received: sample.Received, Sent: sample.Sent, Zxid: sample.Zxid,
	})
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
