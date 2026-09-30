// Package alert 实现预警接收和发送。
// 功能模块自己计算后调用 Submit 写入预警日志。本包不比较阈值，也不读取各功能的指标表。
// 规则表只保存名称、类型、级别、渠道、静默和条件 JSON。本包不解释条件，发送循环不读规则表。
// 后台循环轮询预警日志，按渠道和模板组装后发送，并把结果回写到同一行。
// 管理页是 fg0041 模板、fg0042 渠道、fg0043 日志。0044–0050 号段留着，不各占一页。渠道密钥用 common/crypto 保存。
// 实现按功能分在 models、validate、channel、template、event、service 子包；本文件是对外门面。
package alert

import (
	"fluxguard/server/alert/models"
	alertsvc "fluxguard/server/alert/service"
	"fluxguard/server/alert/template"
	dbx "github.com/fluxsce/dbx/db"
)

// Service 接收预警日志，并按渠道模板发送。
type Service = alertsvc.Service

type (
	Rule          = models.Rule
	RuleInput     = models.RuleInput
	Channel       = models.Channel
	ChannelInput  = models.ChannelInput
	ChannelQuery  = models.ChannelQuery
	Template      = models.Template
	TemplateInput = models.TemplateInput
	TemplateQuery = models.TemplateQuery
	PreviewInput  = models.PreviewInput
	Preview       = models.Preview
	Contact       = models.Contact
	Log           = models.Log
	LogQuery      = models.LogQuery
	Notice        = models.Notice
	IDInput       = models.IDInput
	RuleQuery     = models.RuleQuery
)

// Open 打开预警服务。
func Open(db *dbx.DB) *Service {
	return alertsvc.Open(db)
}

// Current 返回本进程预警服务。尚未 Open 时为 nil。
func Current() *Service {
	return alertsvc.Current()
}

// Render 替换 {{name}}。未识别的占位符保留，便于核对模板。
func Render(text string, vars map[string]string) string {
	return template.Render(text, vars)
}

// DingSign 计算钉钉机器人加签。
func DingSign(secret string, milli int64) (timestamp, sign string) {
	return alertsvc.DingSign(secret, milli)
}
