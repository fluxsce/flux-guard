package config

import (
	"os"
	"testing"
)

func TestMissingFileKeepsListenAndDriver(t *testing.T) {
	cfg, err := Load(t.TempDir() + "/missing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":8080" || cfg.TenantID != "default" || cfg.DB.Driver != "sqlite" || cfg.DB.DSN != "" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.Log.File != "logs/server.log" || !cfg.Log.Console || cfg.Log.MaxSizeMB != 20 {
		t.Fatalf("log=%+v", cfg.Log)
	}
}

func TestLoadDBBlock(t *testing.T) {
	path := t.TempDir() + "/server.yaml"
	body := []byte("listen: \":9090\"\ndb:\n  driver: mysql\n  host: db.example\n  port: 3307\n  username: app\n  password: secret\n  database: guard\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":9090" || cfg.DB.Driver != "mysql" || cfg.DB.Host != "db.example" || cfg.DB.Port != 3307 || cfg.DB.Database != "guard" {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoadHTTPSAndCookieSecure(t *testing.T) {
	path := t.TempDir() + "/server.yaml"
	body := []byte(`listen: ":8443"
enable_https: true
cert_file: certs/a.crt
key_file: certs/a.key
cookie_secure: false
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableHTTPS || cfg.CertFile != "certs/a.crt" || cfg.KeyFile != "certs/a.key" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.SessionCookieSecure() {
		t.Fatal("explicit cookie_secure=false should win")
	}
	cfg.CookieSecure = nil
	if !cfg.SessionCookieSecure() {
		t.Fatal("nil cookie_secure should follow enable_https")
	}
	if _, err := cfg.TLSConfig(); err == nil {
		t.Fatal("missing cert files should fail")
	}
}

func TestTLSConfigDisabled(t *testing.T) {
	cfg := Server{EnableHTTPS: false}
	got, err := cfg.TLSConfig()
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
