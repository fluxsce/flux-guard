package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestExtFrameAndTrace(t *testing.T) {
	call := ExtRequest{V: ExtVersion, ID: "abc12345", TraceID: "abc12345", Method: "probe"}
	if err := CheckExtResponse(call, ExtResponse{V: ExtVersion, ID: call.ID, TraceID: "other123"}); err == nil {
		t.Fatal("trace mismatch")
	}
	encoded, err := EncodeExtResult(map[string]bool{"up": true})
	if err != nil {
		t.Fatal(err)
	}
	reply := ExtResponse{V: ExtVersion, ID: call.ID, TraceID: call.TraceID, OK: true, Result: encoded}
	if err := CheckExtResponse(call, reply); err != nil {
		t.Fatal(err)
	}
	var got map[string]bool
	if err := DecodeExtResult(reply.Result, &got); err != nil || !got["up"] {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	payload := []byte(`{"v":1,"ok":true,"result":"{}"}`)
	if err := WriteExtFrame(&buf, payload); err != nil {
		t.Fatal(err)
	}
	frame, err := ReadExtFrame(&buf)
	if err != nil || string(frame) != string(payload) {
		t.Fatalf("%s %v", frame, err)
	}
	var big bytes.Buffer
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(MaxExtFrame+1))
	_, _ = big.Write(header[:])
	if _, err := ReadExtFrame(&big); !errors.Is(err, ErrExtFrameTooLarge) {
		t.Fatal(err)
	}
}
