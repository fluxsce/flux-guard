package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"fluxguard/protocol"
)

// AppCode 从 WAR 或 JAR 文件名取出应用名和版本。
// order##2.4.1.war 的应用名是 order，版本是 2.4.1。
func AppCode(file string) (name, version string) {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if i := strings.Index(base, "##"); i >= 0 {
		return base[:i], base[i+2:]
	}
	return base, ""
}

// BuiltinWars 是 Tomcat 自带应用，不进入应用管理。
var BuiltinWars = map[string]bool{
	"manager":      true,
	"host-manager": true,
	"docs":         true,
	"examples":     true,
}

// FilesListLimitErr 目录内可列条目超过 protocol.AppFilesListMax。
func FilesListLimitErr() error {
	return fmt.Errorf("文件数量超过 %d，无法列出", protocol.AppFilesListMax)
}

// IsFilesListLimit 判断是否为文件仓列目录超限。
func IsFilesListLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "文件数量超过") && strings.Contains(msg, "无法列出")
}

// EnsureFilesListCap 在追加前检查条数上限。
func EnsureFilesListCap(n int) error {
	if n >= protocol.AppFilesListMax {
		return FilesListLimitErr()
	}
	return nil
}

// FileKind 按扩展名返回制品种类；不认识则空串。
func FileKind(ext string) string {
	switch strings.ToLower(ext) {
	case ".war":
		return "war"
	case ".jar":
		return "jar"
	case ".zip":
		return "static"
	case ".log", ".txt", ".out":
		return "log"
	default:
		return ""
	}
}

// ToProtocol 把内部制品列表转成上报契约。
func ToProtocol(in []Artifact) []protocol.AppArtifact {
	out := make([]protocol.AppArtifact, 0, len(in))
	for _, item := range in {
		out = append(out, protocol.AppArtifact{
			AppName: item.AppName,
			Kind:    item.Kind,
			Version: item.Version,
			Path:    item.Path,
		})
	}
	return out
}
