package files

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"fluxguard/agents/appside/core"
	"fluxguard/protocol"
)

func TestListFilesRejectsOverLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < protocol.AppFilesListMax+1; i++ {
		name := filepath.Join(dir, fmt.Sprintf("f%d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := ListFiles(dir)
	if !core.IsFilesListLimit(err) {
		t.Fatalf("want limit err, got %v", err)
	}
}

func TestListFilesScansWarJarZip(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"datahub01webApp.war", "order##1.2.jar", "pack.zip", "readme.txt", "manager.war"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "child.war"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "app.log"), []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]core.Artifact{}
	for _, item := range got {
		byName[item.AppName] = item
	}
	if byName["datahub01webApp"].Kind != "war" || byName["order"].Kind != "jar" || byName["order"].Version != "1.2" {
		t.Fatalf("%+v", got)
	}
	if byName["pack"].Kind != "static" {
		t.Fatalf("zip %+v", got)
	}
	if byName["nested"].Kind != "dir" {
		t.Fatalf("dir %+v", got)
	}
	if byName["nested/child"].Kind != "war" {
		t.Fatalf("nested war %+v", got)
	}
	if byName["nested/app"].Kind != "log" {
		t.Fatalf("log %+v", got)
	}
	if _, ok := byName["manager"]; ok {
		t.Fatal("builtin war listed")
	}
	if byName["readme"].Kind != "log" {
		t.Fatalf("readme should be log %+v", byName["readme"])
	}
}
