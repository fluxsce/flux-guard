package node

import (
	"encoding/json"
	"fluxguard/agents/appside/core"
	"os"
	"path/filepath"
	"strings"
)

// NodePackage 是 package.json 里用得到的字段。
type NodePackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ReadNodePackage 读取 base 下的 package.json。读不到时返回零值。
func ReadNodePackage(base string) NodePackage {
	body, err := os.ReadFile(filepath.Join(base, "package.json"))
	if err != nil {
		return NodePackage{}
	}
	var pkg NodePackage
	if err := json.Unmarshal(body, &pkg); err != nil {
		return NodePackage{}
	}
	pkg.Name = strings.TrimSpace(pkg.Name)
	pkg.Version = strings.TrimSpace(pkg.Version)
	if len(pkg.Version) > 64 {
		pkg.Version = pkg.Version[:64]
	}
	return pkg
}

// NodeAppName 优先用 package.json 的 name，否则用实例名。
func NodeAppName(rt core.Runtime) string {
	pkg := ReadNodePackage(rt.Base)
	if pkg.Name != "" {
		return pkg.Name
	}
	return rt.Instance
}
