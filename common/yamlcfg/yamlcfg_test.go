package yamlcfg

import (
	"os"
	"testing"
)

func TestReadMissingKeepsDefault(t *testing.T) {
	cfg := struct {
		Listen string `mapstructure:"listen"`
	}{Listen: ":8080"}
	if err := Read(t.TempDir()+"/missing.yaml", &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":8080" {
		t.Fatalf("listen = %q", cfg.Listen)
	}
}

func TestReadYAMLAndEnv(t *testing.T) {
	path := t.TempDir() + "/server.yaml"
	body := []byte("listen: \":8080\"\ndsn: data/flux_guard.db\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLUX_GUARD_LISTEN", ":9090")
	cfg := struct {
		Listen string `mapstructure:"listen"`
		DSN    string `mapstructure:"dsn"`
	}{}
	if err := Read(path, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":9090" || cfg.DSN != "data/flux_guard.db" {
		t.Fatalf("cfg = %+v", cfg)
	}
}
