package plugin_test

import (
	"context"
	"path/filepath"
	"testing"

	"fluxguard/server/component/bridge"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	rt "fluxguard/server/component/runtime"
	"fluxguard/server/plugin"
	"fluxguard/server/store"
)

func TestProbeLoadsManifestAndStores(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, "sqlite", filepath.Join(t.TempDir(), "fg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	root := t.TempDir()
	plugin.InstallHost(t, root, "com.fluxguard.kafka", catalog.Kafka, nil)
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(plugin.Unload)
	if err := plugin.Load(root); err != nil {
		t.Fatal(err)
	}
	attach(t)
	info := findInfo("com.fluxguard.kafka")
	if info.Kind != plugin.KindService || info.BuiltinFlag != "N" || info.Scope != plugin.ScopeServer {
		t.Fatalf("%+v", info)
	}

	target := conn.Open{TenantID: "default", ComponentID: "k1", KindCode: catalog.Kafka, Address: "192.168.1.120:9092", Secret: "hidden-secret"}
	status, err := rt.Probe(ctx, db, target)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Up || status.Version != "helper" || status.Summary != "ok" {
		t.Fatalf("%+v", status)
	}
	points, err := rt.Series(ctx, db, catalog.Kafka, "default", "k1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Values["topicCount"] != 1 || points[0].Values["maxLag"] != 1 {
		t.Fatalf("%+v", points)
	}
	if string(points[0].Detail) != `{"source":"helper"}` {
		t.Fatalf("detail %s", points[0].Detail)
	}
}

func TestMaintainUsesManifestOps(t *testing.T) {
	root := t.TempDir()
	plugin.InstallHost(t, root, "com.fluxguard.zk", catalog.ZK, []plugin.Op{{Name: "mntr"}})
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(plugin.Unload)
	if err := plugin.Load(root); err != nil {
		t.Fatal(err)
	}
	attach(t)
	target := conn.Open{KindCode: catalog.ZK, Address: "127.0.0.1:2181"}
	if _, err := rt.Exec(context.Background(), target, "mntr", map[string]string{"shell": "rm"}); err == nil {
		t.Fatal("expected reject unknown arg")
	}
	if _, err := rt.Exec(context.Background(), target, "create", nil); err == nil {
		t.Fatal("expected reject undeclared op")
	}
	output, err := rt.Exec(context.Background(), target, "mntr", nil)
	if err != nil || output != "mntr" {
		t.Fatalf("%q %v", output, err)
	}
}

func TestFileNameAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	plugin.InstallHost(t, root, "com.fluxguard.files", "", nil)
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(plugin.Unload)
	if err := plugin.Load(root); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := bridge.PutFile(ctx, "com.fluxguard.files", "../note.txt", []byte("hello")); err == nil {
		t.Fatal("expected bad name")
	}
	if err := bridge.PutFile(ctx, "com.fluxguard.files", "note.txt", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	body, err := bridge.GetFile(ctx, "com.fluxguard.files", "note.txt")
	if err != nil || string(body) != "hello" {
		t.Fatalf("%q %v", body, err)
	}
}

func TestProcessStaysUpUntilStop(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, "sqlite", filepath.Join(t.TempDir(), "fg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	root := t.TempDir()
	plugin.InstallHost(t, root, "com.fluxguard.redis", catalog.Redis, nil)
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(plugin.Unload)
	if err := plugin.Load(root); err != nil {
		t.Fatal(err)
	}
	attach(t)
	target := conn.Open{TenantID: "default", ComponentID: "r1", KindCode: catalog.Redis, Address: "127.0.0.1:6379"}
	if status, err := rt.Probe(ctx, db, target); err != nil || !status.Up {
		t.Fatalf("%+v %v", status, err)
	}
	pid1 := pidOf("com.fluxguard.redis")
	if status, err := rt.Probe(ctx, db, target); err != nil || !status.Up {
		t.Fatalf("%+v %v", status, err)
	}
	if pid1 == 0 || pid1 != pidOf("com.fluxguard.redis") {
		t.Fatalf("pid %d %d", pid1, pidOf("com.fluxguard.redis"))
	}
	plugin.Stop("com.fluxguard.redis")
	status, err := rt.Probe(ctx, db, target)
	if err != nil || status.Up || status.Error == "" {
		t.Fatalf("%+v %v", status, err)
	}
	if err := plugin.Start("com.fluxguard.redis"); err != nil {
		t.Fatal(err)
	}
	if status, err := rt.Probe(ctx, db, target); err != nil || !status.Up {
		t.Fatalf("%+v %v", status, err)
	}
}

func TestAttachRejectsUnknownComponent(t *testing.T) {
	root := t.TempDir()
	plugin.InstallHost(t, root, "com.fluxguard.oracle", "oracle", nil)
	t.Setenv("FG_PLUGIN_HOST", "1")
	t.Cleanup(plugin.Unload)
	if err := plugin.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Attach(); err == nil {
		t.Fatal("expected unknown kind")
	}
	if findInfo("com.fluxguard.oracle").Code != "com.fluxguard.oracle" {
		t.Fatal("plugin should stay loaded")
	}
}

func attach(t *testing.T) {
	t.Helper()
	if err := bridge.Attach(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bridge.Detach)
}

func findInfo(code string) plugin.Info {
	for _, info := range plugin.Infos() {
		if info.Code == code {
			return info
		}
	}
	return plugin.Info{}
}

func pidOf(code string) int {
	for _, state := range plugin.States() {
		if state.Code == code {
			return state.PID
		}
	}
	return 0
}
