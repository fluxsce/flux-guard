package sched

import (
	"testing"
	"time"
)

func TestProbeTimeoutFollowsInterval(t *testing.T) {
	if probeTimeout(15) != probeWaitMin {
		t.Fatalf("short interval %s", probeTimeout(15))
	}
	if probeTimeout(60) != time.Minute {
		t.Fatalf("minute interval %s", probeTimeout(60))
	}
	if probeTimeout(86400) != probeWaitMax {
		t.Fatalf("long interval %s", probeTimeout(86400))
	}
}
