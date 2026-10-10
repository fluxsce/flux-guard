package plugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	maxManifest     = 16 * 1024
	methodMaintain  = "maintain"
	idPrefix        = "com.fluxguard."
	protocolFrame   = "length_prefixed_json"
	transportPipe   = "named_pipe"
	transportSocket = "unix_socket"
	startupLazy     = "lazy"
	pipePrefix      = `\\.\pipe\fluxguard.`
)

// Op 是清单里一项维护操作的参数名。中央按这个名字调用 maintain，不会另造方法。
type Op struct {
	Name string   `yaml:"name"`
	Args []string `yaml:"args"`
}

// Method 是一份插件清单里登记的方法。只有 maintain 可以带 Ops。
type Method struct {
	Name string `yaml:"name"`
	Ops  []Op   `yaml:"ops"`
}

// Bridge 说明这份插件接到哪一种组件。ConnectionKind 为空表示不替换内置探针。
type Bridge struct {
	Namespace      string `yaml:"namespace"`
	ConnectionKind string `yaml:"connection_kind"`
}

// Session 说明哪些方法会收到组件口令。口令本身不写进清单。
type Session struct {
	InjectCredentials bool     `yaml:"inject_credentials"`
	CredentialMethods []string `yaml:"credential_methods"`
}

// Runtime 是插件进程怎么启动。三条路径都相对 plugins 根目录，中央按当前系统选一条。
type Runtime struct {
	Executable        string `yaml:"executable"`
	ExecutableWindows string `yaml:"executable_windows"`
	ExecutableUnix    string `yaml:"executable_unix"`
	Lang              string `yaml:"lang"`
}

// IPC 是本机通道。协议只有 length_prefixed_json。地址由中央写进子进程的 FG_PLUGIN_ADDRESS。
type IPC struct {
	Transport        string `yaml:"transport"`
	TransportWindows string `yaml:"transport_windows"`
	TransportUnix    string `yaml:"transport_unix"`
	Address          string `yaml:"address"`
	AddressWindows   string `yaml:"address_windows"`
	AddressUnix      string `yaml:"address_unix"`
	Protocol         string `yaml:"protocol"`
}

// Lifecycle 是进程何时拉起。lazy 表示第一次调用再启动。
type Lifecycle struct {
	Startup string `yaml:"startup"`
}

// Manifest 是 plugins/manifests 里的一份服务清单。
// 分节与可执行文件、通道、凭据注入方式都写在这里。帧格式不在这里。
type Manifest struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Version     string    `yaml:"version"`
	Bridge      Bridge    `yaml:"bridge"`
	Session     Session   `yaml:"session"`
	Runtime     Runtime   `yaml:"runtime"`
	IPC         IPC       `yaml:"ipc"`
	Lifecycle   Lifecycle `yaml:"lifecycle"`
	Permissions []string  `yaml:"permissions"`
	Methods     []Method  `yaml:"methods"`
	Component   string    `yaml:"-"`
	Argv        []string  `yaml:"-"`
	Addr        string    `yaml:"-"`
}

func decodeManifest(root string, raw []byte) (Manifest, map[string]struct{}, map[string]map[string]struct{}, map[string]struct{}, error) {
	if len(raw) == 0 || len(raw) > maxManifest {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单无效")
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单无效")
	}
	var extra struct{}
	if err := dec.Decode(&extra); err == nil {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单无效")
	}
	m.ID = strings.TrimSpace(m.ID)
	m.Name = strings.TrimSpace(m.Name)
	m.Version = strings.TrimSpace(m.Version)
	m.Bridge.Namespace = strings.TrimSpace(m.Bridge.Namespace)
	m.Bridge.ConnectionKind = strings.TrimSpace(m.Bridge.ConnectionKind)
	m.Lifecycle.Startup = strings.TrimSpace(m.Lifecycle.Startup)
	m.Runtime.Lang = strings.TrimSpace(m.Runtime.Lang)
	if !idOK(m.ID) {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件编码无效")
	}
	if m.Name == "" || utf8.RuneCountInString(m.Name) > 64 || hasControl(m.Name) {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件缺少名称")
	}
	if !versionOK(m.Version) {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件缺少版本")
	}
	if !nameOK(m.Bridge.Namespace) {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单里的命名空间无效")
	}
	if m.Bridge.ConnectionKind != "" && m.Bridge.ConnectionKind != m.Bridge.Namespace {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单里的组件种类无效")
	}
	m.Component = m.Bridge.ConnectionKind
	if m.Lifecycle.Startup != startupLazy {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单里的启动方式无效")
	}
	if !langOK(m.Runtime.Lang) {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单里的语言无效")
	}
	if len(m.Permissions) > 0 {
		return Manifest{}, nil, nil, nil, fmt.Errorf("插件清单里的权限无效")
	}
	methods, ops, err := methodsOf(m.Methods)
	if err != nil {
		return Manifest{}, nil, nil, nil, err
	}
	creds, err := credentialsOf(m.Session, methods)
	if err != nil {
		return Manifest{}, nil, nil, nil, err
	}
	if err := resolveRuntime(root, &m); err != nil {
		return Manifest{}, nil, nil, nil, err
	}
	return m, methods, ops, creds, nil
}

func credentialsOf(session Session, methods map[string]struct{}) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(session.CredentialMethods))
	if !session.InjectCredentials {
		if len(session.CredentialMethods) > 0 {
			return nil, fmt.Errorf("插件清单里的凭据方法无效")
		}
		return set, nil
	}
	if len(session.CredentialMethods) == 0 {
		return nil, fmt.Errorf("插件清单里的凭据方法无效")
	}
	for _, name := range session.CredentialMethods {
		name = strings.TrimSpace(name)
		if _, ok := methods[name]; !ok {
			return nil, fmt.Errorf("插件清单里的凭据方法无效")
		}
		if _, ok := set[name]; ok {
			return nil, fmt.Errorf("插件清单里的凭据方法无效")
		}
		set[name] = struct{}{}
	}
	return set, nil
}

func methodsOf(list []Method) (map[string]struct{}, map[string]map[string]struct{}, error) {
	if len(list) == 0 {
		return nil, nil, fmt.Errorf("插件清单里的方法名无效")
	}
	names := make(map[string]struct{}, len(list))
	var ops map[string]map[string]struct{}
	for _, item := range list {
		item.Name = strings.TrimSpace(item.Name)
		if !methodOK(item.Name) {
			return nil, nil, fmt.Errorf("插件清单里的方法名无效")
		}
		if _, ok := names[item.Name]; ok {
			return nil, nil, fmt.Errorf("插件清单里的方法重复")
		}
		names[item.Name] = struct{}{}
		if item.Name != methodMaintain {
			if len(item.Ops) > 0 {
				return nil, nil, fmt.Errorf("只有 maintain 可以声明操作")
			}
			continue
		}
		parsed, err := opsOf(item.Ops)
		if err != nil {
			return nil, nil, err
		}
		ops = parsed
	}
	return names, ops, nil
}

func opsOf(ops []Op) (map[string]map[string]struct{}, error) {
	set := make(map[string]map[string]struct{}, len(ops))
	for _, op := range ops {
		if !nameOK(op.Name) {
			return nil, fmt.Errorf("插件清单里的操作名无效")
		}
		if _, ok := set[op.Name]; ok {
			return nil, fmt.Errorf("插件清单里的操作重复")
		}
		args := make(map[string]struct{}, len(op.Args))
		for _, arg := range op.Args {
			if !nameOK(arg) {
				return nil, fmt.Errorf("插件清单里的参数名无效")
			}
			if _, ok := args[arg]; ok {
				return nil, fmt.Errorf("插件清单里的参数重复")
			}
			args[arg] = struct{}{}
		}
		set[op.Name] = args
	}
	return set, nil
}

func resolveRuntime(root string, m *Manifest) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("插件可执行文件必须在插件目录内")
	}
	// 三条路径都要落在插件目录内。只检查当前系统那一条是否存在。
	for _, part := range []string{m.Runtime.Executable, m.Runtime.ExecutableWindows, m.Runtime.ExecutableUnix} {
		if _, err := insideRoot(rootAbs, part); err != nil {
			return fmt.Errorf("插件可执行文件必须在插件目录内")
		}
	}
	picked := m.Runtime.Executable
	if runtime.GOOS == "windows" && m.Runtime.ExecutableWindows != "" {
		picked = m.Runtime.ExecutableWindows
	}
	if runtime.GOOS != "windows" && m.Runtime.ExecutableUnix != "" {
		picked = m.Runtime.ExecutableUnix
	}
	full, err := insideRoot(rootAbs, picked)
	if err != nil {
		return fmt.Errorf("插件可执行文件必须在插件目录内")
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return fmt.Errorf("插件可执行文件不存在")
	}
	m.Argv = []string{full}
	addr, err := resolveAddress(rootAbs, m.IPC)
	if err != nil {
		return err
	}
	m.Addr = addr
	return nil
}

func resolveAddress(rootAbs string, ipc IPC) (string, error) {
	if ipc.Protocol != protocolFrame {
		return "", fmt.Errorf("插件清单里的协议无效")
	}
	if ipc.Transport != transportPipe || ipc.TransportWindows != transportPipe || ipc.TransportUnix != transportSocket {
		return "", fmt.Errorf("插件清单里的通道无效")
	}
	if ipc.Address != ipc.AddressWindows || !winPipeOK(ipc.AddressWindows) {
		return "", fmt.Errorf("插件清单里的地址无效")
	}
	unixPath, err := insideRoot(rootAbs, ipc.AddressUnix)
	if err != nil || !strings.HasSuffix(ipc.AddressUnix, ".sock") {
		return "", fmt.Errorf("插件清单里的地址无效")
	}
	if runtime.GOOS == "windows" {
		return ipc.AddressWindows, nil
	}
	return unixPath, nil
}

func insideRoot(rootAbs, part string) (string, error) {
	part = strings.TrimSpace(part)
	if part == "" || len(part) > 4096 || filepath.IsAbs(part) || strings.Contains(part, "..") {
		return "", fmt.Errorf("插件可执行文件必须在插件目录内")
	}
	full := filepath.Join(rootAbs, filepath.FromSlash(part))
	rel, err := filepath.Rel(rootAbs, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("插件可执行文件必须在插件目录内")
	}
	return full, nil
}

func winPipeOK(addr string) bool {
	if !strings.HasPrefix(addr, pipePrefix) {
		return false
	}
	rest := addr[len(pipePrefix):]
	if rest == "" || len(rest) > 64 {
		return false
	}
	for _, r := range rest {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func idOK(id string) bool {
	leaf, ok := strings.CutPrefix(id, idPrefix)
	if !ok || strings.Contains(leaf, ".") {
		return false
	}
	return nameOK(leaf)
}

func methodOK(name string) bool {
	if name == "" || len(name) > 32 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	dot := false
	for _, r := range name {
		if r == '.' {
			if dot {
				return false
			}
			dot = true
			continue
		}
		dot = false
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return false
	}
	return !strings.HasSuffix(name, ".")
}

func nameOK(name string) bool {
	if name == "" || len(name) > 32 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func langOK(name string) bool {
	if name == "" || len(name) > 16 {
		return false
	}
	for _, r := range name {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func versionOK(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 8 {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 32 {
			return true
		}
	}
	return false
}
