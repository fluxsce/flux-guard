// Package limits 是本进程超时与体积限制的单一登记处。
//
// 分层约定（勿再把同一数字绑到多种语义上）：
//
//   - HTTP：连接级 ReadHeader / Read / Write / Idle；Read 服务大包上传，与接口超时脱钩。
//   - API：租户可配的 requestTimeoutSeconds，给前端 axios 与管理接口预算；写回 Server 时只调 WriteTimeout。
//   - Agent：长轮询 hold、拨号、在线窗口，固定常量，不跟接口超时共用。
//   - Shell / Alert / Auth：各自独立。
//
// 新增限额：先写进本包并补测试，再在业务包引用；禁止再散落魔法数字。
package limits

import "time"

// --- HTTP 连接（进程级，不可按租户改 Read）---

const (
	// HTTPReadHeader 读请求头时限，防慢头。
	HTTPReadHeader = 5 * time.Second
	// HTTPIdle 空闲 keep-alive。
	HTTPIdle = 60 * time.Second
	// HTTPRead 读完整请求体时限。须覆盖 MaxBodyBytes 上传，与 API 超时分开。
	// 按约 3Mbps 传 200MB 约需 9 分钟，取 10 分钟留余量。
	HTTPRead = 10 * time.Minute
	// HoldWriteMargin 长轮询须在 WriteTimeout 到期前结束，预留这点余量。
	HoldWriteMargin = 2 * time.Second
	// ShutdownWait 优雅退出等待在途请求与后台循环。
	ShutdownWait = 5 * time.Second
)

// --- 请求体体积 ---

const (
	// MaxBodyBytes 管理端请求体绝对上限（安装包等）。
	MaxBodyBytes int64 = 200 << 20
	// MaxJSONBytes JSON / urlencoded 进内存上限。
	MaxJSONBytes int64 = 2 << 20
	// MultipartMemory multipart 先留在内存的字节数，超出落临时文件。
	MultipartMemory int64 = 32 << 20
)

// --- API / 租户访问策略（fg0024 webTimeout）---

const (
	// APIRequestDefaultSec 接口超时默认秒数（前端与写超时预算）。
	APIRequestDefaultSec = 120
	// APIRequestMinSec / APIRequestMaxSec 可配区间。
	APIRequestMinSec = 10
	APIRequestMaxSec = 600
	// SessionDefaultHours 登录会话默认小时数。
	SessionDefaultHours = 12
	SessionMinHours     = 1
	SessionMaxHours     = 168
	// PageSizeFloor / PageSizeCeiling 分页硬边界。
	PageSizeFloor   = 1
	PageSizeCeiling = 200
	// PageSizeDefault 未配置时的默认每页条数。
	PageSizeDefault = 20
)

// --- Agent 命令与拨号 ---

const (
	// CommandWaitHold 代理拉令长轮询默认等待。
	CommandWaitHold = 20 * time.Second
	// DialTimeout 中央拨代理执行的总超时。
	DialTimeout = 12 * time.Second
	// DialConnect TCP 拨号超时。
	DialConnect = 2 * time.Second
	// DialReplyMax 拨号回包最大读取字节（含换包步骤与路径）。
	DialReplyMax = 64 << 10
	// CommandResultNoteMax 命令步骤 JSON 写入 noteText 的上限。
	CommandResultNoteMax = 16 << 10
	// OnlineWindow 心跳窗口内视为在线。
	OnlineWindow = 2 * time.Minute
	// AgentUpgradeBatchMax 单次代理自更新最多同时下发的台数（金丝雀后续批次也受此限）。
	AgentUpgradeBatchMax = 32
	// AgentUpgradeCanaryDefault 未指定金丝雀台数时，首批只升 1 台。
	AgentUpgradeCanaryDefault = 1
	// AppReleaseBatchMax 单次应用换包/回滚最多同时下发的实例数。
	AppReleaseBatchMax = 32
	// AppReleaseCanaryDefault 未指定金丝雀台数时，首批只更新 1 台实例。
	AppReleaseCanaryDefault = 1
	// AppFileDownloadTimeout 中央代下应用代理看守文件的总超时（大 WAR 可能较慢）。
	AppFileDownloadTimeout = 10 * time.Minute
)

// --- Shell 终端 ---

const (
	// ShellInputHold 代理取键盘输入的默认等待。
	ShellInputHold = 12 * time.Second
	// ShellIdle 无读写后回收会话。
	ShellIdle = 90 * time.Second
	// ShellReapEvery 空闲扫描周期。
	ShellReapEvery = 30 * time.Second
	// ShellMaxIn / ShellMaxOut 进程内输入输出缓冲。
	ShellMaxIn  = 64 * 1024
	ShellMaxOut = 256 * 1024
	// ShellMaxInput 单次控制台写入长度。
	ShellMaxInput = 4000
	// ShellMaxPerUser / ShellMaxPerAgent 并发路数。
	ShellMaxPerUser  = 8
	ShellMaxPerAgent = 32
)

// --- 告警 ---

const (
	// AlertScanEvery 预警事件投递扫描周期。各功能检查可沿用同一间隔。
	AlertScanEvery = 30 * time.Second
	// AlertDeliver 单次渠道投递超时（含 SMTP 拨号）。
	AlertDeliver = 8 * time.Second
	// AlertReplyMax 渠道 HTTP 响应读取上限。
	AlertReplyMax = 1024
)

// --- 登录验证码 ---

const (
	// CaptchaTTL 验证码有效期。
	CaptchaTTL = 2 * time.Minute
	// CaptchaClockSkew 允许的时钟偏差。
	CaptchaClockSkew = 5 * time.Second
)

// --- 库表保留期（fg0024 retention；文件日志仍走 common/logger 轮转）---

const (
	// JobTickEvery 任务管理器醒来间隔。是否真正执行看各任务策略。
	JobTickEvery = time.Minute
	// JobTimeoutDefaultSec 策略未写超时时，单次调用的默认秒数。
	JobTimeoutDefaultSec = 600
	// JobTimeoutMinSec / JobTimeoutMaxSec 单次调用超时可配区间。
	JobTimeoutMinSec = 1
	JobTimeoutMaxSec = 86400
	// JobRetryDefault 策略未写重试次数时，失败后再试一次。
	JobRetryDefault = 1
	// JobRetryMax 失败后最多再试几次。
	JobRetryMax = 5
	// JobRetryPause 两次尝试之间的等待。
	JobRetryPause = 2 * time.Second
	// RetentionIntervalDefaultMin 数据归档未配置时，默认每隔这么多分钟跑一轮。
	RetentionIntervalDefaultMin = 60
	// RetentionIntervalMinMin / RetentionIntervalMaxMin 可配间隔区间。
	RetentionIntervalMinMin = 1
	RetentionIntervalMaxMin = 1440
	// RetentionDaysMin / RetentionDaysMax 各类保留天数可配区间；0 表示该类不删。
	RetentionDaysMin = 0
	RetentionDaysMax = 3650
	// RetentionAuditDefaultDays 审计与登录日志默认保留天数。
	RetentionAuditDefaultDays = 90
	// RetentionOpsDefaultDays 终端审计与已完成命令默认保留天数。
	RetentionOpsDefaultDays = 30
	// RetentionAlertDefaultDays 已恢复预警与投递记录默认保留天数。
	RetentionAlertDefaultDays = 90
	// RetentionSampleDefaultDays 主机样本、事件点与上报批次默认保留天数。
	RetentionSampleDefaultDays = 14
	// RetentionDeleteBatch 单次 DELETE 最多删多少行。
	RetentionDeleteBatch = 2000
	// RetentionDeleteMaxBatches 单表单租户单轮最多跑几批，剩余下轮再删。
	RetentionDeleteMaxBatches = 50
	// RetentionDeletePause 批次之间让出锁。
	RetentionDeletePause = 50 * time.Millisecond
)

// ServerWrite 根据接口超时算出 http.Server.WriteTimeout。
// 至少盖住命令/终端长轮询 hold + 余量，避免连接被写超时掐断后业务还在等。
func ServerWrite(apiTimeoutSec int) time.Duration {
	api := time.Duration(apiTimeoutSec) * time.Second
	if apiTimeoutSec <= 0 {
		api = time.Duration(APIRequestDefaultSec) * time.Second
	}
	minWrite := CommandWaitHold + HoldWriteMargin
	if shell := ShellInputHold + HoldWriteMargin; shell > minWrite {
		minWrite = shell
	}
	if api > minWrite {
		return api
	}
	return minWrite
}

// CapHold 把长轮询等待收束到写超时预算内。
func CapHold(want time.Duration, apiTimeoutSec int) time.Duration {
	ceiling := ServerWrite(apiTimeoutSec) - HoldWriteMargin
	if ceiling < time.Second {
		ceiling = time.Second
	}
	if want <= 0 || want > ceiling {
		return ceiling
	}
	if want < time.Second {
		return time.Second
	}
	return want
}

// NormalizeAPITimeoutSec 非法秒数回落到默认。
func NormalizeAPITimeoutSec(sec int) int {
	if sec <= 0 {
		return APIRequestDefaultSec
	}
	return sec
}
