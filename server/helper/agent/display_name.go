package agent

import (
	"strings"
	"unicode/utf8"
)

// IsSystemName 判断是否仍是自动建行时的系统底名。
// 底名形态为种类本身（如 host），或种类加短机器编号（如 appside-a1b2c3d4）。
// 同机多代理靠 agentCode 与 machineId 分行；底名只用于识别「尚未被人改过」。
func IsSystemName(name, agentCode string) bool {
	name = strings.TrimSpace(name)
	agentCode = strings.TrimSpace(agentCode)
	if name == "" {
		return true
	}
	if agentCode == "" {
		return false
	}
	if name == agentCode {
		return true
	}
	prefix := agentCode + "-"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	rest := name[len(prefix):]
	if rest == "" || len(rest) > 64 {
		return false
	}
	return !strings.ContainsAny(rest, " \t\r\n")
}

// ResolveDisplayName 决定上报后要不要改写显示名。
// 手工名、登记名、控制台改过的名一律保留；仅空名或系统底名时，用 preferred
//（reportName / appName）或「种类@主机名」填一次，便于同机多代理在列表里区分。
func ResolveDisplayName(current, agentCode, preferred, hostName string) string {
	current = strings.TrimSpace(current)
	preferred = strings.TrimSpace(preferred)
	hostName = strings.TrimSpace(hostName)
	agentCode = strings.TrimSpace(agentCode)
	if current != "" && !IsSystemName(current, agentCode) {
		return current
	}
	if preferred != "" {
		return clipName(preferred, 100)
	}
	if agentCode != "" && hostName != "" {
		return clipName(agentCode+"@"+hostName, 100)
	}
	if current != "" {
		return current
	}
	return clipName(agentCode, 100)
}

func clipName(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || utf8.RuneCountInString(value) <= max {
		return value
	}
	return string([]rune(value)[:max])
}
