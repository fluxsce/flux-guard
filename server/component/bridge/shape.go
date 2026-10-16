// Package bridge 把插件调用结果接到组件目录上。
// 进程的启动、停止和帧往返在 server/plugin。这里解释 probe、maintain 和文件结果，并写入采集表。
package bridge

import "encoding/json"

const (
	methodProbe    = "probe"
	methodMaintain = "maintain"
	methodFilePut  = "file.put"
	methodFileGet  = "file.get"
	maxFile        = 256 * 1024
	maxDetail      = 32 * 1024
	// maxOutput 是一次维护操作交回的正文上限。采集明细仍用 maxDetail。
	maxOutput = 10 << 20
	maxArgs   = 16
	maxArgLen = 200
	// maxDataLen 是节点正文。maxAclLen 是权限列表。其它参数仍用 maxArgLen。
	maxDataLen = 1 << 20
	maxAclLen  = 64 << 10
)

// ProbeParams 是采集参数。组件口令只放在这里，不进清单。
type ProbeParams struct {
	Kind     string `json:"kind"`
	Address  string `json:"address"`
	User     string `json:"user,omitempty"`
	Secret   string `json:"secret,omitempty"`
	Database string `json:"database,omitempty"`
	Option   string `json:"option,omitempty"`
}

// ProbeResult 是插件交回的采集结果。Values 的键再按组件目录核对。
type ProbeResult struct {
	Up      bool               `json:"up"`
	Version string             `json:"version,omitempty"`
	Role    string             `json:"role,omitempty"`
	Error   string             `json:"error,omitempty"`
	Summary string             `json:"summary,omitempty"`
	Values  map[string]float64 `json:"values,omitempty"`
	Detail  json.RawMessage    `json:"detail,omitempty"`
}

// MethodParams 是一次独立方法调用。Args 的键必须是清单里该方法登记的参数名。
type MethodParams struct {
	Address  string            `json:"address"`
	User     string            `json:"user,omitempty"`
	Secret   string            `json:"secret,omitempty"`
	Database string            `json:"database,omitempty"`
	Option   string            `json:"option,omitempty"`
	Args     map[string]string `json:"args,omitempty"`
}

// MaintainParams 是一项维护操作。Args 的键必须是清单里预先登记的参数名。
type MaintainParams struct {
	Op       string            `json:"op"`
	Address  string            `json:"address"`
	User     string            `json:"user,omitempty"`
	Secret   string            `json:"secret,omitempty"`
	Database string            `json:"database,omitempty"`
	Option   string            `json:"option,omitempty"`
	Args     map[string]string `json:"args,omitempty"`
}

// MaintainResult 是维护操作的文本结果。
type MaintainResult struct {
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// FilePutParams 是交给外置进程的文件。没有路径，也没有组件口令。
type FilePutParams struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Body   string `json:"body"`
}

// FileGetParams 按文件名取回内容。
type FileGetParams struct {
	Name string `json:"name"`
}

// FileResult 是取回的文件，或放入之后的确认。
type FileResult struct {
	OK     bool   `json:"ok,omitempty"`
	Name   string `json:"name,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Body   string `json:"body,omitempty"`
	Error  string `json:"error,omitempty"`
}
