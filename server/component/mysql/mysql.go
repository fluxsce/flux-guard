// Package mysql 用只读查询读取全局状态，并写入 FG_COMPONENT_MYSQL_METRIC。
package mysql

import (
	"context"
	"database/sql"
	"fmt"
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
	mysqldriver "github.com/go-sql-driver/mysql"
)

func init() {
	runtime.Register(catalog.MySQL, probe{})
}

type probe struct{}

// Sample 是一轮 MySQL 采集。复制延迟未知时为 -1。
type Sample struct {
	ThreadsConnected  int64
	ThreadsRunning    int64
	Questions         int64
	SlowQueries       int64
	BufferPoolHitRate float64
	ReplicaLagSec     float64
	UptimeSec         int64
	Version           string
	Role              string
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
	return series.Read(ctx, db, "FG_COMPONENT_MYSQL_METRIC", tenantID, componentID, catalog.Columns(catalog.MySQL), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	addr, err := netx.HostPort(target.Address, 3306)
	if err != nil {
		return Sample{BufferPoolHitRate: -1, ReplicaLagSec: -1}, models.Status{Error: err.Error()}
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return Sample{BufferPoolHitRate: -1, ReplicaLagSec: -1}, models.Status{Error: "地址无效"}
	}
	cfg := mysqldriver.NewConfig()
	cfg.User = target.UserName
	cfg.Passwd = target.Secret
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = target.DatabaseName
	cfg.Timeout = 3 * time.Second
	cfg.ReadTimeout = 5 * time.Second
	cfg.ParseTime = true
	sqlDB, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return Sample{BufferPoolHitRate: -1, ReplicaLagSec: -1}, models.Status{Error: err.Error()}
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.PingContext(ctx); err != nil {
		return Sample{BufferPoolHitRate: -1, ReplicaLagSec: -1}, models.Status{Error: err.Error()}
	}
	fields := map[string]string{}
	rows, err := sqlDB.QueryContext(ctx, "SHOW GLOBAL STATUS")
	if err != nil {
		return Sample{BufferPoolHitRate: -1, ReplicaLagSec: -1}, models.Status{Error: err.Error()}
	}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			_ = rows.Close()
			return Sample{BufferPoolHitRate: -1, ReplicaLagSec: -1}, models.Status{Error: err.Error()}
		}
		fields[name] = value
	}
	_ = rows.Close()
	var version string
	_ = sqlDB.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	sample := ParseStatus(fields)
	sample.Version = version
	sample.ReplicaLagSec = replicaLag(ctx, sqlDB)
	if sample.ReplicaLagSec >= 0 {
		sample.Role = "replica"
	} else {
		sample.Role = "standalone"
	}
	summary := fmt.Sprintf("连接 %d · 活跃 %d", sample.ThreadsConnected, sample.ThreadsRunning)
	if sample.ReplicaLagSec >= 0 {
		summary += fmt.Sprintf(" · 延迟 %.0fs", sample.ReplicaLagSec)
	}
	return sample, models.Status{Up: true, Version: version, Role: sample.Role, Summary: summary}
}

// ParseStatus 从 SHOW GLOBAL STATUS 的名值对取出要入库的数。
func ParseStatus(fields map[string]string) Sample {
	sample := Sample{
		ThreadsConnected:  atoi(fields["Threads_connected"]),
		ThreadsRunning:    atoi(fields["Threads_running"]),
		Questions:         atoi(fields["Questions"]),
		SlowQueries:       atoi(fields["Slow_queries"]),
		UptimeSec:         atoi(fields["Uptime"]),
		BufferPoolHitRate: -1,
		ReplicaLagSec:     -1,
	}
	reads := atof(fields["Innodb_buffer_pool_reads"])
	reqs := atof(fields["Innodb_buffer_pool_read_requests"])
	if reqs > 0 {
		sample.BufferPoolHitRate = (1 - reads/reqs) * 100
		if sample.BufferPoolHitRate < 0 {
			sample.BufferPoolHitRate = 0
		}
	}
	return sample
}

func replicaLag(ctx context.Context, sqlDB *sql.DB) float64 {
	for _, query := range []string{"SHOW REPLICA STATUS", "SHOW SLAVE STATUS"} {
		rows, err := sqlDB.QueryContext(ctx, query)
		if err != nil {
			continue
		}
		cols, err := rows.Columns()
		if err != nil || !rows.Next() {
			_ = rows.Close()
			continue
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			_ = rows.Close()
			continue
		}
		_ = rows.Close()
		for i, name := range cols {
			if name != "Seconds_Behind_Source" && name != "Seconds_Behind_Master" {
				continue
			}
			if vals[i] == nil {
				return -1
			}
			return atof(cellText(vals[i]))
		}
	}
	return -1
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(sample.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(time.Now().UTC()),
		"threadsConnected": sample.ThreadsConnected, "threadsRunning": sample.ThreadsRunning,
		"questions": sample.Questions, "slowQueries": sample.SlowQueries,
		"bufferPoolHitRate": sample.BufferPoolHitRate, "replicaLagSec": sample.ReplicaLagSec,
		"uptimeSec": sample.UptimeSec,
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_COMPONENT_MYSQL_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  threadsConnected, threadsRunning, questions, slowQueries, bufferPoolHitRate, replicaLagSec, uptimeSec,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @threadsConnected, @threadsRunning, @questions, @slowQueries, @bufferPoolHitRate, @replicaLagSec, @uptimeSec,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	if err != nil {
		return err
	}
	return pane.Touch(ctx, db, catalog.MySQL, args)
}

func atoi(v string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n
}

func atof(v string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return n
}

func cellText(v any) string {
	switch typed := v.(type) {
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
