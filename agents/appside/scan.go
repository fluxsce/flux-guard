package appside

import (
	"os"
	"path/filepath"
	"strings"
)

// builtinWars 是 Tomcat 自带应用，不进入应用管理。
var builtinWars = map[string]bool{
	"manager":      true,
	"host-manager": true,
	"docs":         true,
	"examples":     true,
}

// Artifact 是从一套运行时目录里认出来的一个应用。
type Artifact struct {
	AppName string
	Kind    string
	Version string
	Path    string
}

// AppCode 从 WAR 或 JAR 文件名取出应用名和版本。
// order##2.4.1.war 的应用名是 order，版本是 2.4.1。
func AppCode(file string) (name, version string) {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if i := strings.Index(base, "##"); i >= 0 {
		return base[:i], base[i+2:]
	}
	return base, ""
}

// ListWebapps 读取 webapps 下的 WAR。同名已展开目录不另算一条。
// 自带的 manager、host-manager、docs、examples 跳过。
func ListWebapps(dir string) ([]Artifact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Artifact
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(name), ".war") {
			continue
		}
		app, version := AppCode(name)
		if app == "" || builtinWars[app] || seen[app] {
			continue
		}
		seen[app] = true
		out = append(out, Artifact{
			AppName: app,
			Kind:    "war",
			Version: version,
			Path:    filepath.Join(dir, name),
		})
	}
	return out, nil
}

// TomcatVersion 从发行说明的第一行读取版本。读不到时返回空字符串。
func TomcatVersion(base string) string {
	body, err := os.ReadFile(filepath.Join(base, "RELEASE-NOTES"))
	if err != nil {
		return ""
	}
	line := string(body)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	const mark = "Version "
	i := strings.Index(line, mark)
	if i < 0 {
		return ""
	}
	version := strings.TrimSpace(line[i+len(mark):])
	if space := strings.IndexByte(version, ' '); space >= 0 {
		version = version[:space]
	}
	if len(version) > 64 {
		version = version[:64]
	}
	return version
}
