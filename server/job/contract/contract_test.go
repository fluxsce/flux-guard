package contract

import "testing"

func TestKnown(t *testing.T) {
	if !TriggerManual.Known() || !TriggerInterval.Known() || !TriggerAt.Known() || !TriggerCron.Known() {
		t.Fatal("known triggers")
	}
	if Trigger("schedule").Known() {
		t.Fatal("schedule is not a trigger")
	}
}
