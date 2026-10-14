package upgrade

import "testing"

func TestNormalizePlatform(t *testing.T) {
	goos, goarch, err := normalizePlatform("Linux", "AMD64")
	if err != nil || goos != "linux" || goarch != "amd64" {
		t.Fatalf("got %s/%s %v", goos, goarch, err)
	}
	if _, _, err := normalizePlatform("freebsd", "amd64"); err == nil {
		t.Fatal("expected os error")
	}
	if _, _, err := normalizePlatform("linux", "386"); err == nil {
		t.Fatal("expected arch error")
	}
}

func TestSplitBatch(t *testing.T) {
	first, rest := splitBatch([]string{"a", "b", "c"}, 1)
	if len(first) != 1 || first[0] != "a" || len(rest) != 2 {
		t.Fatalf("canary %v %v", first, rest)
	}
	first, rest = splitBatch([]string{"a", "b"}, 8)
	if len(first) != 2 || rest != nil {
		t.Fatalf("all %v %v", first, rest)
	}
}
