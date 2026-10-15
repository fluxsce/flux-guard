package logview

import (
	"strings"
	"testing"
	"time"

	"fluxguard/server/utils/const/limits"
)

func TestPushOutCapsRing(t *testing.T) {
	s := &Store{
		sess: map[string]*session{},
		seen: map[string]time.Time{},
	}
	row := &session{view: View{LogViewID: "x"}}
	chunk := strings.Repeat("a", limits.AppTailMaxPush)
	for i := 0; i < (limits.AppTailMaxOut/limits.AppTailMaxPush)+4; i++ {
		s.pushOut(row, []byte(chunk))
	}
	row.mu.Lock()
	defer row.mu.Unlock()
	if len(row.out) > limits.AppTailMaxOut {
		t.Fatalf("ring %d > max %d", len(row.out), limits.AppTailMaxOut)
	}
	if row.base <= 0 {
		t.Fatalf("expected base advanced, got %d", row.base)
	}
}

func TestPreviewable(t *testing.T) {
	if !previewable("log", "/a/b/c") {
		t.Fatal("kind log should preview")
	}
	if !previewable("", "/var/log/app.log") {
		t.Fatal("ext log should preview")
	}
	if previewable("war", "/webapps/app.war") {
		t.Fatal("war must not preview")
	}
}
