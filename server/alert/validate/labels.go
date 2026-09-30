package validate

// ValidLevel 校验预警级别。
func ValidLevel(code string) bool {
	switch code {
	case "info", "warn", "critical":
		return true
	default:
		return false
	}
}

// LevelName 返回级别中文名。
func LevelName(code string) string {
	switch code {
	case "info":
		return "提示"
	case "warn":
		return "警告"
	case "critical":
		return "严重"
	default:
		return code
	}
}

// ValidKind 校验渠道种类。
func ValidKind(kind string) bool {
	switch kind {
	case "email", "wecom", "dingtalk", "qq", "feishu", "webhook", "sms", "*":
		return true
	default:
		return false
	}
}

// ValidScene 校验发送场景。
func ValidScene(scene string) bool {
	switch scene {
	case "fire", "resolve", "escalate":
		return true
	default:
		return false
	}
}

// SceneName 返回场景中文名。
func SceneName(scene string) string {
	switch scene {
	case "fire":
		return "触发"
	case "resolve":
		return "恢复"
	case "escalate":
		return "升级"
	default:
		return scene
	}
}

// ValidContent 校验正文类型。
func ValidContent(kind string) bool {
	switch kind {
	case "text", "markdown", "html":
		return true
	default:
		return false
	}
}
