package shellrun

import (
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestPTYEcho(t *testing.T) {
	screen, err := openTerm(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer screen.Close()
	enter := "\n"
	if runtime.GOOS == "windows" {
		enter = "\r"
	}
	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		var all []byte
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			n, err := screen.Read(buf)
			all = append(all, buf[:n]...)
			if strings.Contains(string(all), "flux-guard-shell") || err != nil {
				break
			}
		}
		got <- string(all)
	}()
	if _, err := screen.Write([]byte("echo flux-guard-shell" + enter)); err != nil {
		t.Fatal(err)
	}
	select {
	case text := <-got:
		if !utf8.ValidString(text) || !strings.Contains(text, "flux-guard-shell") {
			t.Fatalf("output %q", text)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("控制台没有吐出输出")
	}
}
