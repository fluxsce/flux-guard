-- Flux Guard SQLite 初始化。
-- 与 scripts/db/mysql 同一套表，不含部门。权限通过用户-角色-资源划分。
-- 在本目录执行：sqlite3 flux_guard.db ".read init.sql"
-- SQLite 没有 ON UPDATE，editTime 由应用在写入时更新。
-- 预留列只到 reserved5。

PRAGMA foreign_keys = ON;

.read FG_TENANT.sql
.read FG_USER.sql
.read FG_LOGIN_LOG.sql
.read FG_AUTH_ROLE.sql
.read FG_AUTH_RESOURCE.sql
.read FG_AUTH_USER_ROLE.sql
.read FG_AUTH_ROLE_RESOURCE.sql
.read FG_AUTH_DATA_PERMISSION.sql
.read FG_AUTH_AUDIT_LOG.sql
.read FG_AGENT.sql
.read FG_AGENT_HOST.sql
.read FG_AGENT_JAVA.sql
.read FG_JAVA_HEAP.sql
.read FG_JAVA_GC.sql
.read FG_JAVA_THREAD.sql
.read FG_JAVA_POOL.sql
.read FG_JAVA_RUNTIME.sql
.read FG_JAVA_GC_PAUSE.sql
.read FG_JAVA_THREAD_STATE.sql
.read FG_JAVA_POOL_MAX.sql
.read FG_COMPONENT_KIND.sql
.read FG_COMPONENT_PANEL.sql
.read FG_COMPONENT_CONN.sql
.read FG_COMPONENT_REDIS.sql
.read FG_COMPONENT_MYSQL.sql
.read FG_COMPONENT_MONGO.sql
.read FG_COMPONENT_KAFKA.sql
.read FG_COMPONENT_ZK.sql
.read FG_COMPONENT_NACOS.sql
.read FG_COMPONENT_REDIS_METRIC.sql
.read FG_COMPONENT_MYSQL_METRIC.sql
.read FG_COMPONENT_MONGO_METRIC.sql
.read FG_COMPONENT_KAFKA_METRIC.sql
.read FG_COMPONENT_ZK_METRIC.sql
.read FG_COMPONENT_NACOS_METRIC.sql
.read FG_HOST_CPU.sql
.read FG_HOST_CPU_CORE.sql
.read FG_HOST_MEM.sql
.read FG_HOST_DISK.sql
.read FG_HOST_DISK_IO.sql
.read FG_HOST_NET.sql
.read FG_HOST_TCP.sql
.read FG_HOST_FD.sql
.read FG_HOST_PROC.sql
.read FG_HOST_PROC_TOP.sql
.read FG_HOST_PROC_IO.sql
.read FG_PLUGIN.sql
.read FG_PLUGIN_BINDING.sql
.read FG_INGEST_BATCH.sql
.read FG_EVENT_POINT.sql
.read FG_COMMAND.sql
.read FG_SHELL.sql
.read FG_SHELL_AUDIT.sql
.read FG_SCRIPT_INIT.sql
.read FG_SYS_SETTING.sql
.read FG_JOB.sql
.read FG_JOB_LOG.sql
.read FG_ALERT_RULE.sql
.read FG_HOST_INCIDENT.sql
.read FG_ALERT_TEMPLATE.sql
.read FG_ALERT_CHANNEL.sql
.read FG_ALERT_LOG.sql
