// Package mysql 用只读查询读取全局状态，并写入 FG_COMPONENT_MYSQL_METRIC。
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

func (probe) Probe(ctx context.Context, db *dbx.DB, target conn.Open) (models.Status, error) {
	sample, status := collect(ctx, target)
	now := time.Now().UTC()
	if status.Up && db != nil {
		prev, err := lastCounters(ctx, db, target.TenantID, target.ComponentID)
		if err != nil {
			return status, err
		}
		ApplyRates(&sample, prev, now)
		status.Summary = summaryOf(sample)
	}
	if db != nil {
		if err := save(ctx, db, target.TenantID, target.ComponentID, sample, status, now); err != nil {
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
		return ParseStatus(nil, nil), models.Status{Error: err.Error()}
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ParseStatus(nil, nil), models.Status{Error: "地址无效"}
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
		return ParseStatus(nil, nil), models.Status{Error: err.Error()}
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.PingContext(ctx); err != nil {
		return ParseStatus(nil, nil), models.Status{Error: err.Error()}
	}
	fields, err := nameValues(ctx, sqlDB, "SHOW GLOBAL STATUS")
	if err != nil {
		return ParseStatus(nil, nil), models.Status{Error: err.Error()}
	}
	vars, _ := nameValues(ctx, sqlDB, "SHOW GLOBAL VARIABLES WHERE Variable_name IN ('max_connections', 'innodb_buffer_pool_size')")
	var version string
	_ = sqlDB.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	sample := ParseStatus(fields, vars)
	sample.Version = version
	info, lag := replicaStatus(ctx, sqlDB)
	sample.ReplicaLagSec = lag
	sample.Replica = info
	if info != nil {
		sample.Role = "replica"
		sample.ReplicaIO = ThreadFlag(info.IO)
		sample.ReplicaSQL = ThreadFlag(info.SQL)
	}
	sample.Sessions, sample.SessionNote = sessionsOf(ctx, sqlDB)
	return sample, models.Status{Up: true, Version: version, Role: sample.Role, Summary: summaryOf(sample)}
}

func summaryOf(sample Sample) string {
	text := fmt.Sprintf("连接 %d · 活跃 %d", sample.ThreadsConnected, sample.ThreadsRunning)
	if sample.QuestionsPerSec >= 0 {
		text += fmt.Sprintf(" · %.1f 语句/秒", sample.QuestionsPerSec)
	}
	if sample.ReplicaLagSec >= 0 {
		text += fmt.Sprintf(" · 延迟 %.0f 秒", sample.ReplicaLagSec)
	}
	return text
}

func nameValues(ctx context.Context, sqlDB *sql.DB, query string) (map[string]string, error) {
	rows, err := sqlDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		out[name] = value
	}
	return out, rows.Err()
}

func replicaStatus(ctx context.Context, sqlDB *sql.DB) (*ReplicaInfo, float64) {
	for _, query := range []string{"SHOW REPLICA STATUS", "SHOW SLAVE STATUS"} {
		rows, err := sqlDB.QueryContext(ctx, query)
		if err != nil {
			continue
		}
		info, lag, ok := readReplica(rows)
		_ = rows.Close()
		if ok {
			return info, lag
		}
	}
	return nil, -1
}

func readReplica(rows *sql.Rows) (*ReplicaInfo, float64, bool) {
	cols, err := rows.Columns()
	if err != nil || !rows.Next() {
		return nil, -1, false
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, -1, false
	}
	info := &ReplicaInfo{}
	lag := -1.0
	for i, name := range cols {
		text := ""
		if vals[i] != nil {
			text = strings.TrimSpace(cellText(vals[i]))
		}
		switch name {
		case "Seconds_Behind_Source", "Seconds_Behind_Master":
			if vals[i] != nil {
				lag = atof(text)
			}
		case "Replica_IO_Running", "Slave_IO_Running":
			info.IO = text
		case "Replica_SQL_Running", "Slave_SQL_Running":
			info.SQL = text
		case "Source_Host", "Master_Host":
			if info.Source == "" {
				info.Source = text
			}
		case "Last_IO_Error":
			info.IOError = row.Cut(text, 180)
		case "Last_SQL_Error":
			info.SQLError = row.Cut(text, 180)
		}
	}
	return info, lag, true
}

func sessionsOf(ctx context.Context, sqlDB *sql.DB) ([]Session, string) {
	rows, err := sqlDB.QueryContext(ctx, `
		SELECT ID, USER, HOST, DB, COMMAND, TIME, STATE, LEFT(IFNULL(INFO, ''), 200)
		FROM information_schema.PROCESSLIST
		WHERE COMMAND NOT IN ('Sleep', 'Binlog Dump', 'Binlog Dump GTID', 'Daemon') AND TIME >= 1
		ORDER BY TIME DESC
		LIMIT 32`)
	if err != nil {
		return nil, "读不到正在执行的语句"
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var item Session
		var user, host, dbName, command, state, info sql.NullString
		var id, secs sql.NullInt64
		if err := rows.Scan(&id, &user, &host, &dbName, &command, &secs, &state, &info); err != nil {
			return out, "读不到正在执行的语句"
		}
		item.ID = id.Int64
		item.User = row.Cut(user.String, 64)
		item.Host = row.Cut(host.String, 80)
		item.DB = row.Cut(dbName.String, 64)
		item.Command = row.Cut(command.String, 32)
		item.Time = secs.Int64
		item.State = row.Cut(state.String, 64)
		item.Info = row.Cut(info.String, 200)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return out, "读不到正在执行的语句"
	}
	return out, ""
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status, now time.Time) error {
	ext := ""
	if status.Up {
		ext = mysqlDetail(sample)
	}
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(sample.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(now),
		"threadsConnected": sample.ThreadsConnected, "threadsRunning": sample.ThreadsRunning,
		"questions": sample.Questions, "slowQueries": sample.SlowQueries,
		"bufferPoolHitRate": sample.BufferPoolHitRate, "replicaLagSec": sample.ReplicaLagSec,
		"uptimeSec":       sample.UptimeSec,
		"questionsPerSec": sample.QuestionsPerSec, "slowPerSec": sample.SlowPerSec,
		"selectPerSec": sample.SelectPerSec, "insertPerSec": sample.InsertPerSec,
		"updatePerSec": sample.UpdatePerSec, "deletePerSec": sample.DeletePerSec,
		"netInKbps": sample.NetInKbps, "netOutKbps": sample.NetOutKbps,
		"connUsagePercent": sample.ConnUsagePercent, "maxConnections": sample.MaxConnections,
		"abortedPerSec": sample.AbortedPerSec, "lockWaitsPerSec": sample.LockWaitsPerSec,
		"bufferPoolDirtyPercent": sample.BufferPoolDirtyPercent, "tmpDiskPercent": sample.TmpDiskPercent,
		"replicaIo": sample.ReplicaIO, "replicaSql": sample.ReplicaSQL,
		"extProperty": ext,
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_COMPONENT_MYSQL_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  threadsConnected, threadsRunning, questions, slowQueries, bufferPoolHitRate, replicaLagSec, uptimeSec,
		  questionsPerSec, slowPerSec, selectPerSec, insertPerSec, updatePerSec, deletePerSec,
		  netInKbps, netOutKbps, connUsagePercent, maxConnections, abortedPerSec, lockWaitsPerSec,
		  bufferPoolDirtyPercent, tmpDiskPercent, replicaIo, replicaSql, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @threadsConnected, @threadsRunning, @questions, @slowQueries, @bufferPoolHitRate, @replicaLagSec, @uptimeSec,
		  @questionsPerSec, @slowPerSec, @selectPerSec, @insertPerSec, @updatePerSec, @deletePerSec,
		  @netInKbps, @netOutKbps, @connUsagePercent, @maxConnections, @abortedPerSec, @lockWaitsPerSec,
		  @bufferPoolDirtyPercent, @tmpDiskPercent, @replicaIo, @replicaSql, @extProperty,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	if err != nil {
		return err
	}
	return pane.Touch(ctx, db, catalog.MySQL, args)
}

func mysqlDetail(sample Sample) string {
	doc := map[string]any{"counters": sample.Counters}
	if sample.Replica != nil {
		doc["mysqlReplica"] = sample.Replica
	}
	if len(sample.Sessions) > 0 {
		doc["mysqlSessions"] = sample.Sessions
	}
	if sample.SessionNote != "" {
		doc["mysqlSessionNote"] = sample.SessionNote
	}
	if sample.PoolBytes > 0 {
		doc["mysqlPoolBytes"] = sample.PoolBytes
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return string(body)
}

func lastCounters(ctx context.Context, db *dbx.DB, tenantID, componentID string) (*Prior, error) {
	var raw, observed any
	err := store.Scan(ctx, db, `
		SELECT COALESCE(extProperty, ''), observedTime
		FROM FG_COMPONENT_MYSQL_METRIC
		WHERE tenantId = @tenantId AND componentId = @componentId
		ORDER BY observedTime DESC
		LIMIT 1`, dbx.Args{"tenantId": tenantID, "componentId": componentID}, &raw, &observed)
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
	var wrap struct {
		Counters *Counters `json:"counters"`
	}
	if json.Unmarshal([]byte(row.Text(raw)), &wrap) != nil || wrap.Counters == nil {
		return nil, nil
	}
	return &Prior{Counters: *wrap.Counters, At: at}, nil
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
