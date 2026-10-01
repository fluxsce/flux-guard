// Package zk 用四字命令 mntr 采集，并写入 FG_ZK_METRIC。
package zk

import (
	"context"
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
	return series.Read(ctx, db, "FG_ZK_METRIC", tenantID, componentID, catalog.Columns(catalog.ZK), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	addr, err := netx.HostPort(target.Address, 2181)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	if _, err := io.WriteString(raw, "mntr\n"); err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	body, err := io.ReadAll(io.LimitReader(raw, 64*1024))
	if err != nil && len(body) == 0 {
		return Sample{}, models.Status{Error: err.Error()}
	}
	text := string(body)
	if strings.Contains(text, "not executed") || strings.HasPrefix(strings.TrimSpace(text), "mntr is not") {
		return Sample{}, models.Status{Error: "ZooKeeper 未打开 mntr"}
	}
	sample := ParseMntr(text)
	if sample.Role == "" && sample.ZnodeCount == 0 && sample.Version == "" {
		return sample, models.Status{Error: "mntr 应答无法识别"}
	}
	return sample, models.Status{
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
	}
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(time.Now().UTC()),
		"avgLatency": sample.AvgLatency, "outstandingRequests": sample.OutstandingRequests,
		"znodeCount": sample.ZnodeCount, "followers": sample.Followers, "watchCount": sample.WatchCount,
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_ZK_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  avgLatency, outstandingRequests, znodeCount, followers, watchCount,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @avgLatency, @outstandingRequests, @znodeCount, @followers, @watchCount,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	return err
}

func atoi(v string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n
}

func atof(v string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return n
}
