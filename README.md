<p align="center">
  <strong>现代化主机与应用监控平台</strong>
</p>

<p align="center">
  Flux Guard 是一款基于 Go 语言开发的主机与应用监控平台。代理部署在每台被监控机器上，由中央服务统一接收指标，集成主机观测、Java 运行时、预警通知、远程操作与权限审计等核心能力。提供控制台查看曲线、处置告警并管理任务，帮助运维团队及时掌握运行状况，为主机和应用构建稳定、可观测的监控层。默认 SQLite 即可启动，生产环境可切换 MySQL。
</p>

---

**目录：** [架构](#架构概览) · [功能](#功能) · [快速开始](#快速开始) · [本地开发](#本地开发) · [安装运行](#安装运行) · [文档](#文档)

---

## 架构概览

<p align="center">
  <img src="docs/images/system.svg" alt="Flux Guard 系统架构" width="90%">
</p>

<p align="center"><sub>图 1　部署视图：中央节点、被监控主机与外部通知</sub></p>

<p align="center">
  <img src="docs/images/architecture.svg" alt="Flux Guard 分层架构" width="90%">
</p>

<p align="center"><sub>图 2　逻辑分层：使用层调用控制层，控制层读写数据层，采集层向控制层上报</sub></p>

控制台只访问中央服务。中央服务全局一份，负责保存数据、投递预警，并与各台机器上的代理通信。

| 层 | 职责 |
|----|------|
| 使用层 | 在控制台查看总览、主机、Java 和代理，处理预警，管理权限与任务，查阅操作记录 |
| 控制层 | 全局只部署一份。接收上报、校验身份、保存观测数据、按规则产生预警，并负责命令和交互终端 |
| 数据层 | 保存账号、权限、指标、预警、任务和审计。默认 SQLite，可改用 MySQL |
| 采集层 | 安装在被监控机器上。主机代理采集 CPU、内存、磁盘、网卡和进程；Go 运行时代理只上报自己这个进程；应用代理看守 Tomcat、JAR 或静态目录并换包；Java 代理挂在 Java 进程内，采集堆、垃圾回收和线程 |

一台机器可以同时安装多种代理。其它语言的代理按同一约定上报即可。预警可以送到邮件、企业微信、钉钉、飞书或 Webhook。

图 2 中的编号是层与层之间的交互：

1. 控制台向中央服务查询指标、处置告警，并管理权限与任务。
2. 控制台经中央服务对主机下发命令，或打开交互终端。
3. 中央服务写入并读取账号、指标、预警、任务和审计。
4. 主机代理、Go 运行时代理、应用代理和 Java 代理按间隔上报。同一批数据只保留一次。
5. 命令先由中央服务直接发给主机代理。目标暂时不可达时，改由主机代理来取，执行后回传结果。命令只包含允许的动作。
6. 交互终端默认关闭。打开之后由主机代理主动连回中央服务，主机在内网中也可以使用。终端画面在会话期间转发。

两分钟没有上报的代理会显示为离线。分层、表结构和接入约定见 [架构说明](docs/01-architecture.md)。

---

## 功能

| 能力 | 说明 |
|------|------|
| 主机 | 当前用量、历史曲线，以及按主机设置的预警规则 |
| Java | 堆、垃圾回收和线程 |
| 应用 | 进程自身的运行状况 |
| 预警 | 模板、通知渠道、事件与恢复 |
| 远程操作 | 对主机执行允许的动作，可打开交互终端 |
| 权限 | 按角色授权。页面和其中的操作分别授予 |
| 任务 | 按时间执行内置任务，例如清理过期数据 |
| 审计 | 控制台上的管理操作，以及代理上的命令和终端使用记录 |

---

## 快速开始

需要 **Go 1.25+**、**Node.js 20+**、**pnpm 9+**。安装包的启动方式见 [安装运行](#安装运行)。

| 项 | 值 |
|----|----|
| 控制台 | http://127.0.0.1:5174/fluxguard/ |
| 管理员 | `admin` / `fluxguard` |
| 只读账号 | `viewer` / `fluxguard` |
| 数据库 | 默认 SQLite，文件 `data/flux_guard.db` |

### 本地开发

数据库组件按 `go.mod` 里的 `github.com/fluxsce/dbx` 版本从模块代理拉取，开发不必另克隆一份。要改 dbx 本身、或用上尚未发布的提交时，再把仓库放在同级目录，执行 `go mod edit -replace github.com/fluxsce/dbx=../dbx`。控制台依赖 npm 上的 [`ensis-ui`](https://www.npmjs.com/package/ensis-ui)。

| 组件 | 来源 |
|------|------|
| dbx | https://github.com/fluxsce/dbx （`go.mod` 当前为 v1.0.3） |
| ensis-ui | https://www.npmjs.com/package/ensis-ui |

在 `flux-guard` 根目录执行：

```bash
cp -n configs/server.yaml.example configs/server.yaml
pnpm --dir web install
pnpm dev
```

`pnpm dev` 启动中央服务（`:8080`，读 `configs/server.yaml`）和 Vite（`5174`，`/fluxguard/api` 转到 `8080`）。打开上面的控制台地址登录。首次启动写入数据库和账号。本机 `configs/*.yaml` 留在本机。

Windows 下复制配置：

```powershell
if (-not (Test-Path configs\server.yaml)) { Copy-Item configs\server.yaml.example configs\server.yaml }
```

要看本机上报时，另开终端。先从对应的 `configs/*.yaml.example` 复制出 `host.yaml`、`goruntime.yaml` 或 `appside.yaml`，再执行 `go run ./cmd/host`、`go run ./cmd/goruntime` 或 `go run ./cmd/appside`。开发上报令牌是 `agt_dev_local`。Java 代理源码在 `agents/java`，配置是 `configs/java.yaml`。应用代理一台机器一份，在 `runtimes` 里列出各套 Tomcat、JAR 和静态目录，进程和应用分开。Go 运行时代理只上报自己这个进程，同一台机器上的多份进程用 `report_name` 区分。

`go test ./...` 跑后端测试。`pnpm --dir web run typecheck` 检查控制台。监听地址和数据库写在 `configs/server.yaml`，也可用前缀 `FLUX_GUARD` 的环境变量覆盖。

### 安装运行

安装包解开后是 `flux-guard/`。中央服务在根目录，用 `scripts/run` 启动和停止。各代理已经各自打成压缩包，放在 `agents/` 里，拷到目标机器再解压：

| 压缩包 | 解开后的目录 |
|---|---|
| `agents/flux-guard-agent-host.tar.gz` | `flux-guard-agent-host/` |
| `agents/flux-guard-agent-goruntime.tar.gz` | `flux-guard-agent-goruntime/` |
| `agents/flux-guard-agent-appside.tar.gz` | `flux-guard-agent-appside/` |
| `agents/flux-guard-agent-java.tar.gz` | `flux-guard-agent-java/` |

主机代理、Go 运行时代理和应用代理的目录里有程序、配置示例和启动停止脚本。Java 代理目录里有 `flux-guard-agent-java.jar` 和配置示例，挂载方式写在 `configs/java.yaml.example` 的注释里。第一次启动时，若还没有本机 `configs/*.yaml`，这些启动脚本会从示例复制一份。Java 代理没有启动脚本，把示例复制为 `configs/java.yaml` 后再改。程序只读这份 yaml，不读 example。本机配置留在机器上，不要提交。

中央服务、主机代理、Go 运行时代理和应用代理要在机器重启后自动起来，在各自目录执行一次 `scripts/run/install.sh`（Windows 用管理员执行 `install.ps1`）。这会登记成系统服务并立即启动：Linux 是 systemd，macOS 是 launchd，Windows 是开机任务。服务不把程序输出送进系统日志，日志仍写在安装目录的 `logs/` 里并按大小轮转。重启用同目录的 `restart.sh` 或 `restart.ps1`。取消开机启动用 `uninstall.sh` 或 `uninstall.ps1`。Java 代理不登记。

在中央目录执行这次安装时，会把同包的 `agents/flux-guard-agent-host.tar.gz` 解到 `agents/flux-guard-agent-host/`，再调用该目录自己的安装脚本。主机指标仍由这个独立进程采集，中央进程不读本机 CPU、内存和磁盘。`configs/host.yaml` 的 `server_url` 按中央 `listen` 写成这台机器能访问到的地址：监听所有网卡时用 `127.0.0.1`，绑了具体地址时用那个地址。初次令牌与开发种子 `agt_dev_local` 相同。在环境设置里更换上报令牌后，要同步改这份 `agent_token`。再次安装会按当前 `listen` 改 `server_url`，不改已有令牌和显示名。卸载中央服务时一并取消这个主机代理的开机启动，不删除已解开的目录和配置。其它机器上的代理仍各自拷包安装。安装包里没有这只主机包时，中央服务会装上，本机主机代理不会装，安装脚本以失败结束。

安装包带有控制台页面。浏览器打开 `http://主机:listen端口/fluxguard/`，例如 `listen` 为 `:8080` 时是 `http://127.0.0.1:8080/fluxguard/`。根路径会转到这个地址。

监听地址和数据库可以在服务配置中修改，也可以用前缀为 `FLUX_GUARD` 的环境变量覆盖。

---

## 文档

| 文档 | 说明 |
|------|------|
| [架构说明](docs/01-architecture.md) | 分层、数据表、命令与终端通道、接入约定 |
| [更新日志](CHANGELOG.md) | 版本记录 |
