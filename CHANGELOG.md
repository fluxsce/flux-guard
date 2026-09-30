# 更新日志

本文件记录产品版本。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，并遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

`go.mod` 里的 `go` 行是语言版本。表字段 `currentVersion` 是行的乐观锁，不在这里。

## [Unreleased]

## [1.0.1] - 2026-09-30

### 新增
- 安装包的下载文件名仍带版本和平台。解开后的目录固定为 `flux-guard/`，目录名不带版本号。中央服务放在这个根目录，并用 `scripts/run` 启动和停止。Linux、macOS 使用 shell，Windows 使用 PowerShell。目录里只有一个程序时，直接执行脚本即可。
- 主机、应用和 Java 代理各自再打成压缩包，放在 `agents/`，整包拷到目标机器后再解压。文件是 `flux-guard-agent-host.tar.gz`、`flux-guard-agent-app.tar.gz`、`flux-guard-agent-java.tar.gz`。解开后的目录分别是 `flux-guard-agent-host/`、`flux-guard-agent-app/`、`flux-guard-agent-java/`，同样不带版本号。主机包和应用包内含程序、配置示例和同一套启动停止脚本。Java 包只有配置示例，用 `-javaagent` 挂进目标进程。
- 仓库和安装包提供 `configs/*.yaml.example`。第一次启动时，若本机还没有对应的 `configs/*.yaml`，启动脚本从示例复制一份，已有文件不覆盖。

### 变更
- 中央服务、主机代理、应用代理和 Java 代理只读取本机 `configs/*.yaml`。同目录的 example 不会被当作配置加载。
- 本机 `configs/*.yaml` 留在本机，不进入 Git。

## [1.0.0] - 2026-09-30

### 新增
- 任务调度按 Job 接口调用：手动、按间隔、指定时间、任意五段 cron。同一任务执行中不重叠，单次调用有超时，失败按策略重试。参数对象由业务解释；结果写入 `FG_JOB` / `FG_JOB_LOG`。可登记多个任务。控制台按任务列表查看、修改、执行；同一种类可新增，也可填写已登记的自定义种类并自己写 JSON 参数。内置任务不能删除。每条任务可查看历史执行日志和下一次开始时间。归档范围写在策略 JSON 的 params 里。
- 系统任务管理器（`server/job` + fg0025）：可登记内置/外部任务，统一策略（`job:{code}`）、状态与调度；数据归档为首个内置执行器（`server/retention`）。
- 统一库表归档清理：按 Dataset 分批硬删审计、登录、终端、已完成命令、已恢复预警、投递、样本与上报批次。
- 日志写入 `log.file`，按大小和天数轮转，并限制旧文件份数。
- 数据库配置挂在 `db` 下。`dsn` 仍可直接填写；留空时用主机、端口、账号、口令和库名拼连接串。
- 产品版本记在仓库根 `package.json`，并同步到 `common/buildinfo`。本地构建、`-version` 和主机代理上报使用这个号；发行包再用 ldflags 写入提交号和构建时间。控制台登录页、侧栏和账号菜单显示当前服务版本。
- GitHub 流水线：推送到 `master` 或拉取请求时跑 `go test`。`package.json` 的版本尚未发布且更新日志里有对应条目时，构建完成后打标签 `vX.Y.Z`，并发布 server、host、app 安装包。
