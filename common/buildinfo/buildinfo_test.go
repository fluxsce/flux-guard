package buildinfo

import (
	"strings"
	"testing"
)

func TestDetailIncludesVersion(t *testing.T) {
	line := Detail()
	if !strings.Contains(line, Version) || !strings.Contains(line, Commit) {
		t.Fatal(line)
	}
}
