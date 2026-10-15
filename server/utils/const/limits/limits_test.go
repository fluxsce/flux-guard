package limits

import (
	"testing"
	"time"
)

func TestServerWriteCoversLongPoll(t *testing.T) {
	if got := ServerWrite(10); got < CommandWaitHold+HoldWriteMargin {
		t.Fatalf("write %v too short for command hold", got)
	}
	// 须盖住应用代下把大文件流给浏览器的时间。
	want := AppFileDownloadTimeout + HoldWriteMargin
	if got := ServerWrite(APIRequestDefaultSec); got != want {
		t.Fatalf("default write %v want %v", got, want)
	}
	if got := ServerWrite(0); got != want {
		t.Fatalf("zero write %v want %v", got, want)
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
	if AppFileMaxBytes < 1<<30 {
		t.Fatalf("app file cap %d must cover 1GiB", AppFileMaxBytes)
	}
	if AppFileMaxBytes <= MaxBodyBytes {
		t.Fatal("app file push must exceed management body cap")
	}
}
