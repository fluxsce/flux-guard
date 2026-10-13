package hostvol

import (
	"regexp"
	"strings"
)

// 分区不是一块独立的盘。iostat 默认也不列出它们，读写已经算在整盘上。
var partitionName = regexp.MustCompile(`(?i)^(?:(?:hd|sd|vd|xvd)[a-z]+\d+|(?:nvme\d+n\d+|mmcblk\d+|nbd\d+|rbd\d+|loop\d+)p\d+)$`)

// 容器运行时把卷绑到宿主机上，容量和图例都不是一块物理盘。
var markers = []string{
	"/volume-subpaths/",
	"/var/lib/kubelet/",
	"/var/lib/docker/",
	"/var/lib/containerd/",
	"/var/lib/containers/",
	"/var/lib/rancher/",
	"/var/lib/lxc/",
	"/var/lib/crio/",
	"/run/containerd/",
	"/run/docker/",
}

// Container 判断挂载点是不是容器卷。
func Container(mount string) bool {
	path := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(mount), `\`, "/"))
	if path == "" {
		return false
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	for _, marker := range markers {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

// Partition 判断名字是不是磁盘分区，例如 vda1、sda2、nvme0n1p1。
// nvme0n1、dm-0、md0 是整盘。容量仍按分区看，这里只用于磁盘 IO。
func Partition(name string) bool {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "/dev/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return partitionName.MatchString(name)
}

// BlockName 用块设备名标识一块盘。去掉 /dev/ 前缀；没有设备名时退回挂载点。
func BlockName(device, mount string) string {
	name := strings.TrimSpace(device)
	name = strings.TrimPrefix(name, "/dev/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	return strings.TrimSpace(mount)
}

// SkipSQL 生成排除容器卷的条件。column 只能是调用方自己的列名。
func SkipSQL(column string) string {
	var b strings.Builder
	for _, marker := range markers {
		b.WriteString(" AND LOWER(")
		b.WriteString(column)
		b.WriteString(") NOT LIKE '%")
		b.WriteString(strings.Trim(marker, "/"))
		b.WriteString("%'")
	}
	return b.String()
}
