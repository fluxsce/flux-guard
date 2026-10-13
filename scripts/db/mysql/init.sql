-- Flux Guard MySQL 初始化。
-- 在本目录执行：mysql -u... flux_guard < init.sql 不会展开 source。
-- 请在 mysql 客户端里 cd 到本目录后执行 source init.sql。
-- 建表不写 CHARSET/COLLATE，继承当前库。
-- 命名对齐 gateway：表名大写、字段 camelCase、禁止自增、禁止字段名 id、开关只用 Y/N。
-- 新表预留列只到 reserved5，宽度分档，避免 utf8mb4 行宽超限。

source FG_TENANT.sql;
source FG_USER.sql;
source FG_LOGIN_LOG.sql;
source FG_AUTH_ROLE.sql;
source FG_AUTH_RESOURCE.sql;
source FG_AUTH_USER_ROLE.sql;
source FG_AUTH_ROLE_RESOURCE.sql;
source FG_AUTH_DATA_PERMISSION.sql;
source FG_AUTH_AUDIT_LOG.sql;
source FG_AGENT.sql;
source FG_AGENT_HOST.sql;
source FG_AGENT_JAVA.sql;
source FG_JAVA_RUN.sql;
source FG_JAVA_HEAP.sql;
source FG_JAVA_GC.sql;
source FG_JAVA_THREAD.sql;
source FG_JAVA_POOL.sql;
source FG_JAVA_RUNTIME.sql;
source FG_JAVA_GC_PAUSE.sql;
source FG_JAVA_THREAD_STATE.sql;
source FG_JAVA_POOL_MAX.sql;
source FG_COMPONENT_KIND.sql;
source FG_COMPONENT_PANEL.sql;
source FG_COMPONENT_CONN.sql;
source FG_COMPONENT_REDIS.sql;
source FG_COMPONENT_MYSQL.sql;
source FG_COMPONENT_MONGO.sql;
source FG_COMPONENT_KAFKA.sql;
source FG_COMPONENT_ZK.sql;
source FG_COMPONENT_NACOS.sql;
source FG_COMPONENT_REDIS_METRIC.sql;
source FG_COMPONENT_MYSQL_METRIC.sql;
source FG_COMPONENT_MONGO_METRIC.sql;
source FG_COMPONENT_KAFKA_METRIC.sql;
source FG_COMPONENT_ZK_METRIC.sql;
source FG_COMPONENT_NACOS_METRIC.sql;
source FG_HOST_CPU.sql;
source FG_HOST_CPU_CORE.sql;
source FG_HOST_MEM.sql;
source FG_HOST_DISK.sql;
source FG_HOST_DISK_IO.sql;
source FG_HOST_NET.sql;
source FG_HOST_TCP.sql;
source FG_HOST_FD.sql;
source FG_HOST_PROC.sql;
source FG_HOST_PROC_TOP.sql;
source FG_HOST_PROC_IO.sql;
source FG_PLUGIN.sql;
source FG_PLUGIN_BINDING.sql;
source FG_INGEST_BATCH.sql;
source FG_EVENT_POINT.sql;
source FG_COMMAND.sql;
source FG_SHELL.sql;
source FG_SHELL_AUDIT.sql;
source FG_SCRIPT_INIT.sql;
source FG_SYS_SETTING.sql;
source FG_JOB.sql;
source FG_JOB_LOG.sql;
source FG_ALERT_RULE.sql;
source FG_HOST_INCIDENT.sql;
source FG_ALERT_TEMPLATE.sql;
source FG_ALERT_CHANNEL.sql;
source FG_ALERT_LOG.sql;
