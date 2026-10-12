package store

import (
	"strings"
	"testing"

	"fluxguard/server/config"
)

func TestResolveSQLiteDSN(t *testing.T) {
	driver, dsn, err := Resolve(config.DB{Driver: "sqlite", DSN: "data/flux_guard.db", Host: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != "data/flux_guard.db" {
		t.Fatalf("%s %s", driver, dsn)
	}
}

func TestResolveSQLiteFile(t *testing.T) {
	driver, dsn, err := Resolve(config.DB{Driver: "sqlite", Database: "data/app.db"})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != "data/app.db" {
		t.Fatalf("%s %s", driver, dsn)
	}
}

func TestResolveMySQLHost(t *testing.T) {
	driver, dsn, err := Resolve(config.DB{
		Driver: "mysql", Host: "127.0.0.1", Port: 3306,
		Username: "root", Password: "p@ss", Database: "flux_guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "mysql" || !strings.Contains(dsn, "tcp(127.0.0.1:3306)/flux_guard") {
		t.Fatalf("%s %s", driver, dsn)
	}
	if !strings.Contains(dsn, "parseTime=true") || strings.Contains(dsn, "loc=Local") {
		t.Fatal(dsn)
	}
	if got := Describe(driver, dsn); got != "mysql 127.0.0.1:3306/flux_guard" {
		t.Fatal(got)
	}
}

func TestResolveMySQLRequiresHost(t *testing.T) {
	if _, _, err := Resolve(config.DB{Driver: "mysql", Database: "flux_guard"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveEmptyUsesSQLiteFile(t *testing.T) {
	driver, dsn, err := Resolve(config.DB{})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != "data/flux_guard.db" {
		t.Fatalf("%s %s", driver, dsn)
	}
}
