// Package buildinfo 保存产品版本。
// 默认号来自仓库根 package.json，见 PackageVersion。
// 发行构建覆盖：go build -ldflags "-X fluxguard/common/buildinfo.Version=1.2.3 -X fluxguard/common/buildinfo.Commit=abc -X fluxguard/common/buildinfo.Date=2026-09-27T00:00:00Z"
// 监听地址和数据库在 server/config，不在这里。
package buildinfo

import (
	"fmt"
	"os"
)

var (
	Version = PackageVersion
	Commit  = "none"
	Date    = "unknown"
)

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
