//go:build windows

package shellrun

import (
	"testing"
	"time"
)

func TestCloseUnblocksRead(t *testing.T) {
	screen, err := openTerm(80, 24)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 256)
		for {
			_, err := screen.Read(buf)
			if err != nil {
				close(done)
				return
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)
	if err := screen.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("关闭时读仍卡住")
	}
}
