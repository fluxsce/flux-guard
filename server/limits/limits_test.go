package limits

import (
	"testing"
	"time"
)

func TestServerWriteCoversLongPoll(t *testing.T) {
	if got := ServerWrite(10); got < CommandWaitHold+HoldWriteMargin {
		t.Fatalf("write %v too short for command hold", got)
	}
	if got := ServerWrite(APIRequestDefaultSec); got != time.Duration(APIRequestDefaultSec)*time.Second {
		t.Fatalf("default write %v", got)
	}
	if got := ServerWrite(0); got != time.Duration(APIRequestDefaultSec)*time.Second {
		t.Fatalf("zero write %v", got)
	}
}

func TestCapHold(t *testing.T) {
	if got := CapHold(CommandWaitHold, APIRequestDefaultSec); got != CommandWaitHold {
		t.Fatalf("hold %v", got)
	}
	if got := CapHold(time.Hour, 30); got != ServerWrite(30)-HoldWriteMargin {
		t.Fatalf("capped %v", got)
	}
}

func TestBodyLayers(t *testing.T) {
	if MaxJSONBytes >= MaxBodyBytes {
		t.Fatal("json cap must be below body cap")
	}
	if MultipartMemory > MaxBodyBytes {
		t.Fatal("multipart memory must fit under body cap")
	}
}
