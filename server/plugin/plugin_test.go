package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"fluxguard/protocol"
	"fluxguard/server/component/catalog"
)

const (
	methodProbe   = "probe"
	methodFilePut = "file.put"
	methodFileGet = "file.get"
)

func TestMain(m *testing.M) {
	if os.Getenv("FG_PLUGIN_HOST") == "1" {
		helper()
		return
	}
	os.Exit(m.Run())
}

func helper() {
	addr := os.Getenv(pluginAddrEnv)
	if addr == "" {
		os.Exit(1)
	}
	ln, err := listenLocal(addr)
	if err != nil {
		os.Exit(1)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// 每条连接自己处理，慢采集不会挡住同一进程上的其它调用。
		go helperConn(conn)
	}
}

func helperConn(conn net.Conn) {
	defer conn.Close()
	for {
		payload, err := protocol.ReadExtFrame(conn)
		if err != nil {
			return
		}
		var call protocol.ExtRequest
		if err := json.Unmarshal(payload, &call); err != nil || call.ID == "" || call.TraceID == "" {
			return
		}
		reply := protocol.ExtResponse{V: protocol.ExtVersion, ID: call.ID, TraceID: call.TraceID, OK: true}
		switch call.Method {
		case methodProbe:
			var params struct {
				Kind string `json:"kind"`
				Wait bool   `json:"wait"`
			}
			_ = json.Unmarshal(call.Params, &params)
			if params.Wait {
				time.Sleep(300 * time.Millisecond)
			}
			values := map[string]float64{}
			for _, col := range catalog.Columns(params.Kind) {
				values[col] = 1
			}
			reply.Result, err = protocol.EncodeExtResult(map[string]any{
				"up": true, "version": "helper", "role": "plugin", "summary": "ok",
				"values": values, "detail": json.RawMessage(`{"source":"helper"}`),
			})
		case methodMaintain:
			var params struct {
				Op string `json:"op"`
			}
			_ = json.Unmarshal(call.Params, &params)
			reply.Result, err = protocol.EncodeExtResult(map[string]string{"output": params.Op})
		case methodFilePut:
			var params struct {
				Name   string `json:"name"`
				SHA256 string `json:"sha256"`
				Body   string `json:"body"`
			}
			_ = json.Unmarshal(call.Params, &params)
			body, decErr := base64.StdEncoding.DecodeString(params.Body)
			if decErr != nil || sha256Hex(body) != params.SHA256 || !hostFileName(params.Name) {
				reply.OK = false
				reply.Error = "文件校验失败"
				reply.ErrorCode = protocol.ExtCodeInvalidParams
			} else {
				reply.Result, err = protocol.EncodeExtResult(map[string]any{"ok": true, "name": params.Name})
			}
		case methodFileGet:
			var params struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(call.Params, &params)
			body := []byte("hello")
			if params.Name != "note.txt" {
				reply.OK = false
				reply.Error = "文件名无效"
				reply.ErrorCode = protocol.ExtCodeInvalidParams
			} else {
				reply.Result, err = protocol.EncodeExtResult(map[string]string{
					"name": params.Name, "sha256": sha256Hex(body), "body": base64.StdEncoding.EncodeToString(body),
				})
			}
		default:
			reply.OK = false
			reply.Error = "这种组件没有这项操作"
			reply.ErrorCode = protocol.ExtCodeMethodNotFound
		}
		if err != nil {
			return
		}
		out, err := json.Marshal(reply)
		if err != nil || protocol.WriteExtFrame(conn, out) != nil {
			return
		}
	}
}

func TestConcurrentCalls(t *testing.T) {
	root := t.TempDir()
	installHost(t, root, "com.fluxguard.kafka", catalog.Kafka, nil)
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(Unload)
	if err := Load(root); err != nil {
		t.Fatal(err)
	}
	var warm map[string]any
	if err := Call(context.Background(), "com.fluxguard.kafka", methodProbe, map[string]any{"kind": catalog.Kafka}, "", &warm); err != nil {
		t.Fatal(err)
	}
	const n = 4
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dest map[string]any
			errCh <- Call(context.Background(), "com.fluxguard.kafka", methodProbe, map[string]any{"kind": catalog.Kafka, "wait": true}, "", &dest)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed > 900*time.Millisecond {
		t.Fatalf("calls were serialized: %s", elapsed)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	raw := []byte("id: com.fluxguard.kafka\nname: Kafka\nshell: true\n")
	if _, _, _, _, err := decodeManifest(t.TempDir(), raw); err == nil {
		t.Fatal("expected closed manifest")
	}
}

func TestLoadRejectsCommandOutsideDir(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "com.fluxguard.kafka", catalog.Kafka, nil, "../probe")
	if err := Load(root); err == nil {
		t.Fatal("expected path reject")
	}
	assertGone(t, "com.fluxguard.kafka")
}

func TestLoadRejectsDuplicateCode(t *testing.T) {
	root := t.TempDir()
	installHost(t, root, "com.fluxguard.demo", "", nil)
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(Unload)
	if err := Load(root); err != nil {
		t.Fatal(err)
	}
	if err := Load(root); err == nil {
		t.Fatal("expected duplicate code")
	}
	info := findInfo("com.fluxguard.demo")
	if info.Kind != KindService || info.BuiltinFlag != "N" {
		t.Fatalf("%+v", info)
	}
	if pidOf("com.fluxguard.demo") != 0 {
		t.Fatal("lazy plugin should stay stopped until called")
	}
}

func TestLoadRollsBackWhenComponentRepeats(t *testing.T) {
	root := t.TempDir()
	installHost(t, root, "com.fluxguard.kafka_a", catalog.Kafka, nil)
	installHost(t, root, "com.fluxguard.kafka_b", catalog.Kafka, nil)
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(Unload)
	if err := Load(root); err == nil {
		t.Fatal("expected duplicate component")
	}
	assertGone(t, "com.fluxguard.kafka_a")
	assertGone(t, "com.fluxguard.kafka_b")
}

func TestCatalogReportsIdlePlugin(t *testing.T) {
	root := t.TempDir()
	installHost(t, root, "com.fluxguard.kafka", catalog.Kafka, []Op{{Name: "create_topic", Args: []string{"name"}}})
	t.Cleanup(Unload)
	if err := Load(root); err != nil {
		t.Fatal(err)
	}
	var item CatalogItem
	found := false
	for _, row := range Catalog() {
		if row.Code == "com.fluxguard.kafka" {
			item = row
			found = true
		}
	}
	if !found {
		t.Fatal("missing catalog row")
	}
	if item.Version != "1.0.0" || item.Component != catalog.Kafka || item.Lang != "go" || item.Startup != startupLazy || item.Status != CatalogIdle || item.PID != 0 {
		t.Fatalf("%+v", item)
	}
	if len(item.Methods) < 2 || item.Methods[0].Name != methodProbe || item.Methods[1].Name != methodMaintain || len(item.Methods[1].Ops) != 1 || item.Methods[1].Ops[0] != "create_topic" {
		t.Fatalf("%+v", item.Methods)
	}
}

func TestMissingPluginDir(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal(err)
	}
}

func installHost(t *testing.T, root, code, component string, ops []Op) {
	t.Helper()
	dir := filepath.Join(root, code)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "host"
	if runtime.GOOS == "windows" {
		name = "host.exe"
	}
	dest := filepath.Join(dir, name)
	// 不能硬链接正在运行的测试程序。Windows 会锁住所有硬链接，临时目录删不掉。
	src, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, root, code, component, ops, filepath.ToSlash(filepath.Join(code, name)))
}

func writeManifest(t *testing.T, root, code, component string, ops []Op, executable string) {
	t.Helper()
	dir := filepath.Join(root, "manifests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	leaf := strings.TrimPrefix(code, idPrefix)
	ns := leaf
	if component != "" {
		ns = component
	}
	body, err := yaml.Marshal(Manifest{
		ID: code, Name: leaf, Version: "1.0.0",
		Bridge:    Bridge{Namespace: ns, ConnectionKind: component},
		Session:   Session{InjectCredentials: true, CredentialMethods: []string{methodProbe, methodMaintain}},
		Runtime:   Runtime{Executable: executable, ExecutableWindows: executable, ExecutableUnix: executable, Lang: "go"},
		IPC:       IPC{Transport: transportPipe, TransportWindows: transportPipe, TransportUnix: transportSocket, Address: pipePrefix + leaf, AddressWindows: pipePrefix + leaf, AddressUnix: leaf + ".sock", Protocol: protocolFrame},
		Lifecycle: Lifecycle{Startup: startupLazy},
		Methods: []Method{
			{Name: methodProbe},
			{Name: methodMaintain, Ops: ops},
			{Name: methodFilePut},
			{Name: methodFileGet},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, code+".yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func findInfo(code string) Info {
	for _, info := range Infos() {
		if info.Code == code {
			return info
		}
	}
	return Info{}
}

func pidOf(code string) int {
	for _, state := range States() {
		if state.Code == code {
			return state.PID
		}
	}
	return 0
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func hostFileName(name string) bool {
	if strings.Contains(name, "..") {
		return false
	}
	return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`).MatchString(name)
}

func assertGone(t *testing.T, code string) {
	t.Helper()
	if findInfo(code).Code != "" {
		t.Fatalf("%s still listed", code)
	}
	for _, state := range States() {
		if state.Code == code && state.PID != 0 {
			t.Fatalf("%s still running %+v", code, state)
		}
	}
}
