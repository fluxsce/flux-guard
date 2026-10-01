package hostnic

import "strings"

// 容器运行时和 CNI 建在宿主机上的网卡。流量已经出现在物理网卡上，不能再单独计入主机合计。
var prefixes = []string{
	"veth",
	"cali",
	"lxc",
	"azv",
	"docker",
	"br-",
	"cni",
	"flannel",
	"tunl",
	"vxlan.calico",
	"cilium",
	"weave",
	"kube-ipvs",
	"nodelocaldns",
	"ovn-k8s",
}

// Container 判断网卡是不是容器或 CNI 网卡。
func Container(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return false
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

// SkipSQL 生成排除容器网卡的条件。column 只能是调用方自己的列名。
func SkipSQL(column string) string {
	var b strings.Builder
	for _, prefix := range prefixes {
		b.WriteString(" AND LOWER(")
		b.WriteString(column)
		b.WriteString(") NOT LIKE '")
		b.WriteString(prefix)
		b.WriteString("%'")
	}
	return b.String()
}
