package buildinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetailIncludesVersion(t *testing.T) {
	line := Detail()
	if !strings.Contains(line, Version) || !strings.Contains(line, Commit) {
		t.Fatal(line)
	}
}

func TestPackageVersionMatchesRootPackage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Version != PackageVersion {
		t.Fatalf("package.json %s != PackageVersion %s; run node scripts/sync-version.mjs", pkg.Version, PackageVersion)
	}
}
