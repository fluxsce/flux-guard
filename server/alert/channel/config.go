package channel

import (
	"encoding/json"
	"strings"

	"fluxguard/common/crypto"
	"fluxguard/server/alert/models"
	"fluxguard/server/alert/template"
	"fluxguard/server/alert/validate"
)

var secretKeys = []string{"password", "secret", "token", "accessToken"}

func isSecretKey(key string) bool {
	switch key {
	case "password", "secret", "token", "accessToken":
		return true
	default:
		return false
	}
}

// ParseConfig 解析渠道 JSON 配置。
func ParseConfig(text string) map[string]any {
	text = strings.TrimSpace(text)
	if text == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func writeConfig(cfg map[string]any) (string, error) {
	cleaned := map[string]any{}
	for key, value := range cfg {
		if isSecretKey(key) && strings.TrimSpace(validate.AsText(value)) == "" {
			continue
		}
		if strings.TrimSpace(validate.AsText(value)) == "" && !isSecretKey(key) {
			if _, ok := value.(map[string]any); !ok {
				if _, ok := value.([]any); !ok {
					continue
				}
			}
		}
		cleaned[key] = value
	}
	body, err := json.Marshal(cleaned)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func setSecret(cfg map[string]any, key, plain string) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return
	}
	enc, err := crypto.Encrypt("", plain)
	if err != nil {
		cfg[key] = plain
		return
	}
	cfg[key] = enc
}

// PlainSecret 从配置中解密密钥字段。
func PlainSecret(cfg map[string]any, key string) string {
	value := strings.TrimSpace(validate.AsText(cfg[key]))
	if value == "" {
		return ""
	}
	if !crypto.IsEncrypted(value) {
		return value
	}
	out, err := crypto.Decrypt("", value)
	if err != nil {
		return ""
	}
	return out
}

func applyChannel(old map[string]any, in models.ChannelInput) (map[string]any, error) {
	cfg := map[string]any{}
	for _, key := range secretKeys {
		if value := strings.TrimSpace(validate.AsText(old[key])); value != "" {
			cfg[key] = old[key]
		}
	}
	switch in.ChannelKind {
	case "email":
		cfg["host"] = strings.TrimSpace(in.Endpoint)
		if in.Port > 0 {
			cfg["port"] = in.Port
		}
		cfg["username"] = strings.TrimSpace(in.Username)
		cfg["from"] = strings.TrimSpace(in.Sender)
		cfg["to"] = strings.TrimSpace(in.Target)
		cfg["tls"] = validate.Flag(in.OptionText, "N")
		setSecret(cfg, "password", in.Secret)
		if strings.TrimSpace(validate.AsText(cfg["host"])) == "" {
			return nil, validate.ErrText("请填写 SMTP 主机")
		}
	case "wecom", "dingtalk", "feishu":
		cfg["webhook"] = strings.TrimSpace(in.Endpoint)
		setSecret(cfg, "secret", in.Secret)
		if strings.TrimSpace(validate.AsText(cfg["webhook"])) == "" {
			return nil, validate.ErrText("请填写机器人 Webhook")
		}
	case "qq":
		cfg["endpoint"] = strings.TrimSpace(in.Endpoint)
		cfg["targetId"] = strings.TrimSpace(in.Target)
		option := strings.TrimSpace(in.OptionText)
		if option == "" {
			option = "group"
		}
		if option != "group" && option != "private" {
			return nil, validate.ErrText("QQ 目标类型只能是 group 或 private")
		}
		cfg["targetType"] = option
		setSecret(cfg, "accessToken", in.Secret)
		if strings.TrimSpace(validate.AsText(cfg["endpoint"])) == "" {
			return nil, validate.ErrText("请填写 QQ 机器人地址")
		}
	case "webhook":
		cfg["url"] = strings.TrimSpace(in.Endpoint)
		setSecret(cfg, "token", in.Secret)
		if strings.TrimSpace(validate.AsText(cfg["url"])) == "" {
			return nil, validate.ErrText("请填写 Webhook 地址")
		}
	case "sms":
		cfg["url"] = strings.TrimSpace(in.Endpoint)
		cfg["to"] = strings.TrimSpace(in.Target)
		setSecret(cfg, "token", in.Secret)
		if strings.TrimSpace(validate.AsText(cfg["url"])) == "" {
			return nil, validate.ErrText("请填写短信网关地址")
		}
	default:
		return nil, validate.ErrText("渠道种类无效")
	}
	if err := mergeExtra(cfg, in.ExtraText); err != nil {
		return nil, err
	}
	templateID := strings.TrimSpace(in.TemplateID)
	if templateID == "" {
		templateID = template.BuiltinID(in.ChannelKind)
	}
	if templateID != "" {
		cfg["templateId"] = templateID
	}
	return cfg, nil
}

func mergeExtra(cfg map[string]any, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var extra map[string]any
	if err := json.Unmarshal([]byte(text), &extra); err != nil || extra == nil {
		return validate.ErrText("扩展配置必须是 JSON 对象")
	}
	for key, value := range extra {
		if isSecretKey(key) {
			plain := strings.TrimSpace(validate.AsText(value))
			if plain == "" {
				continue
			}
			setSecret(cfg, key, plain)
			continue
		}
		cfg[key] = value
	}
	return nil
}

func channelFromConfig(base models.Channel, cfg map[string]any) models.Channel {
	switch base.ChannelKind {
	case "email":
		base.Endpoint = strings.TrimSpace(validate.AsText(cfg["host"]))
		base.Port = validate.AsInt(cfg["port"])
		base.Username = strings.TrimSpace(validate.AsText(cfg["username"]))
		base.Sender = strings.TrimSpace(validate.AsText(cfg["from"]))
		base.Target = strings.TrimSpace(validate.AsText(cfg["to"]))
		base.OptionText = validate.Flag(validate.AsText(cfg["tls"]), "N")
	case "wecom", "dingtalk", "feishu":
		base.Endpoint = strings.TrimSpace(validate.AsText(cfg["webhook"]))
	case "qq":
		base.Endpoint = strings.TrimSpace(validate.AsText(cfg["endpoint"]))
		base.Target = strings.TrimSpace(validate.AsText(cfg["targetId"]))
		base.OptionText = strings.TrimSpace(validate.AsText(cfg["targetType"]))
	case "webhook", "sms":
		base.Endpoint = strings.TrimSpace(validate.AsText(cfg["url"]))
		base.Target = strings.TrimSpace(validate.AsText(cfg["to"]))
	}
	base.TemplateID = strings.TrimSpace(validate.AsText(cfg["templateId"]))
	if base.TemplateID == "" {
		base.TemplateID = template.BuiltinID(base.ChannelKind)
	}
	base.SecretSet = "N"
	for _, key := range secretKeys {
		if strings.TrimSpace(validate.AsText(cfg[key])) != "" {
			base.SecretSet = "Y"
			break
		}
	}
	extra := map[string]any{}
	known := map[string]struct{}{
		"host": {}, "port": {}, "username": {}, "password": {}, "from": {}, "to": {}, "tls": {},
		"webhook": {}, "secret": {}, "endpoint": {}, "targetId": {}, "targetType": {},
		"url": {}, "token": {}, "accessToken": {},
		"templateId": {}, "subject": {}, "body": {}, "contentType": {},
	}
	for key, value := range cfg {
		if _, skip := known[key]; skip || isSecretKey(key) {
			continue
		}
		extra[key] = value
	}
	if len(extra) > 0 {
		if body, err := json.Marshal(extra); err == nil {
			base.ExtraText = string(body)
		}
	}
	return base
}
