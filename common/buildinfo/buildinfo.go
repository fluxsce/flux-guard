// Package buildinfo 保存产品版本。
// 本地和测试从仓库根 package.json 读取 version。发行构建用 ldflags 覆盖 Version、Commit、Date。
// 监听地址和数据库在 server/config，不在这里。
package buildinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func init() {
	// 发行包已经写入提交号，沿用 ldflags 里的版本。
	if Commit != "none" {
		return
	}
	if version := readPackageVersion(); version != "" {
		Version = version
	}
}

// Detail 是启动日志和 -version 打印的一行。
func Detail() string {
	return fmt.Sprintf("%s commit=%s built=%s", Version, Commit, Date)
}

// ExitIfRequested 在参数含 -version 或 --version 时打印版本并退出。
func ExitIfRequested() {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" {
			fmt.Println(Detail())
			os.Exit(0)
		}
	}
}

// readPackageVersion 从当前目录或程序所在目录向上查找本仓库的 package.json。
func readPackageVersion() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		if version := walkPackageVersion(start); version != "" {
			return version
		}
	}
	return ""
}

func walkPackageVersion(start string) string {
	dir := start
	for range 8 {
		if version := packageVersionAt(filepath.Join(dir, "package.json")); version != "" {
			return version
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func packageVersionAt(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &pkg); err != nil {
		return ""
	}
	if pkg.Name != "flux-guard" || pkg.Version == "" {
		return ""
	}
	return pkg.Version
}
