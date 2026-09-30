package template

import "fluxguard/server/alert/models"

// 库里的内置模板由 scripts/db 种子写入。下面只保留发送/预览时的编码约定与兜底文案。

type builtinTemplate struct {
	id      string
	name    string
	kind    string
	content string
	subject string
	body    string
}

func builtinTemplates() []builtinTemplate {
	kinds := []struct {
		kind string
		name string
	}{
		{"email", "邮件"},
		{"wecom", "企业微信"},
		{"dingtalk", "钉钉"},
		{"qq", "QQ"},
		{"feishu", "飞书"},
		{"webhook", "Webhook"},
		{"sms", "短信"},
	}
	out := make([]builtinTemplate, 0, len(kinds))
	for _, kind := range kinds {
		content, subject, body := defaultCopy(kind.kind)
		out = append(out, builtinTemplate{
			id: "tpl-" + kind.kind, name: kind.name, kind: kind.kind,
			content: content, subject: subject, body: body,
		})
	}
	return out
}

// BuiltinID 返回渠道种类对应的内置模板编码。
func BuiltinID(kind string) string {
	for _, item := range builtinTemplates() {
		if item.kind == kind {
			return item.id
		}
	}
	return ""
}

func defaultCopy(kind string) (content, subject, body string) {
	subject = "[{{level}}] {{scene}} {{ruleName}} {{hostName}}"
	plain := "主机 {{hostName}}（{{agentId}}）\n指标 {{metricName}} 当前 {{value}}，阈值 {{operator}} {{threshold}}。\n{{message}}"
	short := "{{level}} {{hostName}} {{metricName}} {{value}}，阈值 {{operator}} {{threshold}}"
	markdown := "**{{level}} {{scene}} {{ruleName}}**\n> 主机：{{hostName}}\n> 指标：{{metricName}} {{value}}（阈值 {{operator}} {{threshold}}）\n> {{message}}"
	html := "<p><b>{{level}} {{scene}} {{ruleName}}</b></p><p>主机 {{hostName}}（{{agentId}}）</p><p>指标 {{metricName}} 当前 {{value}}，阈值 {{operator}} {{threshold}}。</p><p>{{message}}</p>"
	switch kind {
	case "email":
		return "html", subject, html
	case "wecom", "dingtalk":
		return "markdown", subject, markdown
	case "sms":
		return "text", subject, short
	default:
		return "text", subject, plain
	}
}

// FallbackTemplate 返回渠道种类的内置模板副本。
func FallbackTemplate(kind string) models.Template {
	for _, item := range builtinTemplates() {
		if item.kind == kind {
			return models.Template{
				TemplateID: item.id, TemplateName: item.name, ChannelKind: kind,
				SubjectText: item.subject, BodyText: item.body, ContentType: item.content,
				BuiltInFlag: "Y", StatusFlag: "Y", ActiveFlag: "Y",
			}
		}
	}
	content, subject, body := defaultCopy("webhook")
	return models.Template{
		TemplateName: "通用", ChannelKind: kind,
		SubjectText: subject, BodyText: body, ContentType: content, StatusFlag: "Y", ActiveFlag: "Y",
	}
}
