package template

import (
	"encoding/json"
	"regexp"

	"fluxguard/server/alert/models"
	"fluxguard/server/alert/validate"
)

var placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// Render 替换 {{name}}。未识别的占位符保留，便于核对模板。
func Render(text string, vars map[string]string) string {
	if text == "" || vars == nil {
		return text
	}
	return placeholder.ReplaceAllStringFunc(text, func(all string) string {
		match := placeholder.FindStringSubmatch(all)
		if len(match) != 2 {
			return all
		}
		if value, ok := vars[match[1]]; ok {
			return value
		}
		return all
	})
}

// SampleVars 返回预览用的样例变量。
func SampleVars() map[string]string {
	return map[string]string{
		"ruleName":     "CPU 使用率过高",
		"level":        "严重",
		"levelCode":    "critical",
		"metric":       "cpu.usage",
		"metricName":   "主机 CPU 使用率",
		"value":        "95",
		"threshold":    "80",
		"operator":     ">",
		"hostName":     "web-01",
		"agentId":      "agent-local",
		"firedAt":      "2026-09-28 15:04:05",
		"status":       "触发",
		"message":      "主机 web-01 主机 CPU 使用率 当前 95，阈值 > 80",
		"scene":        "触发",
		"contactNames": "张三",
	}
}

// MessageVars 从预警日志和扩展 JSON 构造模板变量。扩展里的同名键覆盖默认值。
func MessageVars(item models.Log) map[string]string {
	vars := map[string]string{
		"ruleName":     item.AlertTitle,
		"level":        validate.LevelName(item.AlertLevel),
		"levelCode":    item.AlertLevel,
		"message":      item.AlertContent,
		"scene":        validate.SceneName(item.SceneCode),
		"status":       validate.SceneName(item.SceneCode),
		"firedAt":      item.AlertTimestamp,
		"contactNames": "",
	}
	var extra map[string]string
	if json.Unmarshal([]byte(item.AlertExtra), &extra) == nil {
		for key, value := range extra {
			if value != "" {
				vars[key] = value
			}
		}
	}
	return vars
}
