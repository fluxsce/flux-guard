package core

import "strings"

// DescribeLifecycle 把本机启停配置写成一句回执说明。不执行命令。
// action 取 start、stop、restart。
func DescribeLifecycle(rt Runtime, action string) string {
	switch strings.TrimSpace(action) {
	case "stop":
		return describeStop(rt)
	case "start":
		return describeStart(rt)
	default:
		return describeRestart(rt)
	}
}

func describeRestart(rt Runtime) string {
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		return "systemctl restart " + strings.TrimSpace(rt.Service)
	case "windows":
		return "sc stop 后 sc start " + strings.TrimSpace(rt.Service)
	case "docker":
		return "docker restart " + strings.TrimSpace(rt.Service)
	case "command":
		if script := strings.TrimSpace(rt.Restart); script != "" {
			return "只执行重启脚本：" + clipCmd(script)
		}
		return describeCommandStop(rt) + "然后" + describeStart(rt)
	default:
		return "不认识的运行方式 " + strings.TrimSpace(rt.Kind)
	}
}

func describeStart(rt Runtime) string {
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		return "systemctl start " + strings.TrimSpace(rt.Service)
	case "windows":
		return "sc start " + strings.TrimSpace(rt.Service)
	case "docker":
		return "docker start " + strings.TrimSpace(rt.Service)
	case "command":
		if s := strings.TrimSpace(rt.Start); s != "" {
			return "执行启动脚本：" + clipCmd(s)
		}
		return "未配置启动脚本"
	default:
		return "不认识的运行方式 " + strings.TrimSpace(rt.Kind)
	}
}

func describeStop(rt Runtime) string {
	switch strings.TrimSpace(rt.Kind) {
	case "systemd":
		return "systemctl stop " + strings.TrimSpace(rt.Service)
	case "windows":
		return "sc stop " + strings.TrimSpace(rt.Service)
	case "docker":
		return "docker stop " + strings.TrimSpace(rt.Service)
	case "command":
		return describeCommandStop(rt)
	default:
		return "不认识的运行方式 " + strings.TrimSpace(rt.Kind)
	}
}

func describeCommandStop(rt Runtime) string {
	var b strings.Builder
	if stop := strings.TrimSpace(rt.Stop); stop != "" {
		b.WriteString("先执行停止脚本：")
		b.WriteString(clipCmd(stop))
		b.WriteString("。")
	} else {
		b.WriteString("未配置停止脚本。")
	}
	b.WriteString(killPhrase(rt))
	return b.String()
}

func killPhrase(rt Runtime) string {
	if rt.KillAuto || (strings.TrimSpace(rt.Mode) == "tomcat" && strings.TrimSpace(rt.Kill) == "") {
		base := strings.TrimSpace(rt.Base)
		if base == "" {
			return "仍在跑则按 catalina.base 强杀。"
		}
		return "仍在跑则按 -Dcatalina.base=" + base + " 强杀。"
	}
	if kill := strings.TrimSpace(rt.Kill); kill != "" {
		return "仍在跑则执行强杀：" + clipCmd(kill) + "。"
	}
	return "未配置强杀。"
}

func clipCmd(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return s
}
