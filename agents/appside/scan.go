package appside

import (
	"fmt"
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

// Artifact 是从一套运行时目录里认出来的一个应用或目录。
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

const filesMaxDepth = 8

// ListInstanceFiles 按配置的 instance 名列出该套 files 模式目录下的制品（供中央短拨）。
func ListInstanceFiles(list []Runtime, instance string) ([]Artifact, error) {
	rt, ok := FindRuntime(list, instance)
	if !ok {
		return nil, fmt.Errorf("找不到实例 %s", instance)
	}
	if rt.Mode != "files" {
		return nil, fmt.Errorf("实例 %s 不是 files 模式", instance)
	}
	return ListFiles(rt.Base)
}

// ListFiles 递归读取制品仓：子目录、war/jar/zip，以及常见日志文件（便于后期下日志）。
// AppName 对嵌套文件带相对目录，例如 nested/order，保证同一套实例内唯一。
// 供控制台打开时短拨调用；心跳不再全量登记。
func ListFiles(dir string) ([]Artifact, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var out []Artifact
	seen := map[string]bool{}
	err = walkFiles(abs, abs, 0, &out, seen)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func walkFiles(root, dir string, depth int, out *[]Artifact, seen map[string]bool) error {
	if depth > filesMaxDepth {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		rel, err := filepath.Rel(root, full)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			app := rel
			if seen[app] {
				continue
			}
			seen[app] = true
			*out = append(*out, Artifact{AppName: app, Kind: "dir", Path: full})
			if err := walkFiles(root, full, depth+1, out, seen); err != nil {
				return err
			}
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		kind := fileKind(ext)
		if kind == "" {
			continue
		}
		base, version := AppCode(name)
		if base == "" {
			continue
		}
		if kind == "war" && builtinWars[base] && !strings.Contains(rel, "/") {
			continue
		}
		app := base
		if parent := filepath.ToSlash(filepath.Dir(rel)); parent != "." && parent != "" {
			app = parent + "/" + base
		}
		if seen[app] {
			continue
		}
		seen[app] = true
		*out = append(*out, Artifact{
			AppName: app, Kind: kind, Version: version, Path: full,
		})
	}
	return nil
}

func fileKind(ext string) string {
	switch ext {
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
