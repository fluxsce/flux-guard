package core

import (
	"bytes"
	"path/filepath"
	"strings"
)

// catalinaBaseArg 是 Tomcat JVM 的完整参数。比较时按整段参数相等，不用实例名做子串。
func catalinaBaseArg(base string) string {
	base = filepath.Clean(strings.TrimSpace(base))
	if base == "" || base == "." {
		return ""
	}
	return "-Dcatalina.base=" + base
}

// cmdlineHasExactArg 判断 NUL 分隔的命令行里是否有与 arg 完全相同的一段。
// classpath 里出现 base 目录、或另一个更长的 catalina.base，都不算。
func cmdlineHasExactArg(cmdline []byte, arg string) bool {
	if arg == "" || len(cmdline) == 0 {
		return false
	}
	rest := cmdline
	for len(rest) > 0 {
		i := bytes.IndexByte(rest, 0)
		var field []byte
		if i < 0 {
			field = rest
			rest = nil
		} else {
			field = rest[:i]
			rest = rest[i+1:]
		}
		if string(field) == arg {
			return true
		}
	}
	return false
}

func tomcatBaseAlive(rt Runtime) (alive bool, handled bool) {
	if strings.TrimSpace(rt.Mode) != "tomcat" || !rt.StatusAuto {
		return false, false
	}
	arg := catalinaBaseArg(rt.Base)
	if arg == "" {
		return false, false
	}
	return processHasExactArg(arg)
}

// killTomcatBase 在本机能按 base 强杀时返回 true，并已发出信号。否则交给调用方执行 Kill 命令。
func killTomcatBase(rt Runtime) bool {
	if strings.TrimSpace(rt.Mode) != "tomcat" || !rt.KillAuto {
		return false
	}
	arg := catalinaBaseArg(rt.Base)
	if arg == "" {
		return false
	}
	return killExactArg(arg)
}
