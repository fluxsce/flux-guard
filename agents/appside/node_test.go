package appside

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadNodePackage(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "package.json"), []byte(`{"name":"order-api","version":"2.1.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := ReadNodePackage(base)
	if pkg.Name != "order-api" || pkg.Version != "2.1.0" {
		t.Fatalf("%+v", pkg)
	}
	rt := Runtime{Base: base, Instance: "fallback"}
	if NodeAppName(rt) != "order-api" {
		t.Fatal(NodeAppName(rt))
	}
}
