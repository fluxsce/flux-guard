// Package retention 统一清理库里的过期日志与样本。
// 文件日志轮转仍在 common/logger；本包只硬删 FG_* 追加写表。
// 对齐 gateway：按 Dataset 声明表与截止时间，模块不再各自写 DELETE。
package retention

import (
	"context"
	"time"

	"fluxguard/server/job/ext/archive"
	"fluxguard/server/store"
)

// Dataset 一类可过期数据的声明。调度删除共用这份清单。
type Dataset struct {
	Name       string
	Table      string
	TimeCol    string
	ExtraWhere string // 可选附加条件，例如只删已完成命令。字面量，不拼用户输入。
	Cutoff     func(ctx context.Context, tenantID string) (time.Time, bool)
}

// DaysCutoff 按保留天数计算截止时间。天数 <= 0 则跳过。
func DaysCutoff(daysFor func(context.Context, string) int) func(context.Context, string) (time.Time, bool) {
	return func(ctx context.Context, tenantID string) (time.Time, bool) {
		if daysFor == nil {
			return time.Time{}, false
		}
		days := daysFor(ctx, tenantID)
		if days <= 0 {
			return time.Time{}, false
		}
		return time.Now().UTC().AddDate(0, 0, -days), true
	}
}

// Datasets 返回当前支持硬删的全部数据集。
func Datasets() []Dataset {
	audit := DaysCutoff(func(ctx context.Context, tenantID string) int {
		return archive.Days(ctx, tenantID).AuditLogDays
	})
	ops := DaysCutoff(func(ctx context.Context, tenantID string) int {
		return archive.Days(ctx, tenantID).OpsLogDays
	})
	alert := DaysCutoff(func(ctx context.Context, tenantID string) int {
		return archive.Days(ctx, tenantID).AlertDays
	})
	sample := DaysCutoff(func(ctx context.Context, tenantID string) int {
		return archive.Days(ctx, tenantID).SampleDays
	})
	list := []Dataset{
		{Name: "audit-log", Table: "FG_AUTH_AUDIT_LOG", TimeCol: "addTime", Cutoff: audit},
		{Name: "login-log", Table: "FG_LOGIN_LOG", TimeCol: "addTime", Cutoff: audit},
		{Name: "shell-audit", Table: "FG_SHELL_AUDIT", TimeCol: "addTime", Cutoff: ops},
		{
			Name: "command-log", Table: "FG_COMMAND", TimeCol: "addTime", Cutoff: ops,
			// 只删已结束的命令，避免清掉等待/执行中的行。
			ExtraWhere: "statusFlag IN ('Y', 'N')",
		},
		{Name: "alert-log", Table: "FG_ALERT_LOG", TimeCol: "addTime", Cutoff: alert},
		{
			Name: "host-incident", Table: "FG_HOST_INCIDENT", TimeCol: "addTime", Cutoff: alert,
			ExtraWhere: "statusCode = 'resolved'",
		},
		{Name: "job-log", Table: "FG_JOB_LOG", TimeCol: "addTime", Cutoff: ops},
		{Name: "event-point", Table: "FG_EVENT_POINT", TimeCol: "addTime", Cutoff: sample},
		{Name: "ingest-batch", Table: "FG_INGEST_BATCH", TimeCol: "addTime", Cutoff: sample},
	}
	for _, table := range hostSampleTables {
		list = append(list, Dataset{
			Name:    "host-" + table,
			Table:   table,
			TimeCol: "addTime",
			Cutoff:  sample,
		})
	}
	for _, table := range javaSampleTables {
		list = append(list, Dataset{
			Name:    "java-" + table,
			Table:   table,
			TimeCol: "addTime",
			Cutoff:  sample,
		})
	}
	return list
}

// hostSampleTables 是滚动主机样本。FG_HOST_RULE 是配置表，不在此列。
var hostSampleTables = []string{
	"FG_HOST_CPU",
	"FG_HOST_CPU_CORE",
	"FG_HOST_MEM",
	"FG_HOST_DISK",
	"FG_HOST_DISK_IO",
	"FG_HOST_NET",
	"FG_HOST_FD",
	"FG_HOST_TCP",
	"FG_HOST_PROC",
	"FG_HOST_PROC_TOP",
}

// javaSampleTables 是滚动 Java 样本。FG_AGENT_JAVA 是身份表，不在此列。
var javaSampleTables = []string{
	"FG_JAVA_HEAP",
	"FG_JAVA_GC",
	"FG_JAVA_THREAD",
	"FG_JAVA_POOL",
	"FG_JAVA_RUNTIME",
	"FG_JAVA_GC_PAUSE",
	"FG_JAVA_THREAD_STATE",
	"FG_JAVA_POOL_MAX",
}

// formatBefore 把截止时间写成库里用的时间文本。
func formatBefore(t time.Time) string {
	return store.FormatTime(t)
}
