package command

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fluxguard/protocol"
)

func TestListenRequiresToken(t *testing.T) {
	handler := Handler("secret", func(context.Context, protocol.CommandCall) (string, error) {
		return "ok", nil
	})
	body := `{"commandId":"c1","action":"host.collect.now"}`

	missing := httptest.NewRequest(http.MethodPost, protocol.ExecPath, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, missing)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token %d", rec.Code)
	}

	wrong := httptest.NewRequest(http.MethodPost, protocol.ExecPath, strings.NewReader(body))
	wrong.Header.Set("Authorization", protocol.AuthorizationValue("other"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, wrong)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token %d", rec.Code)
	}

	ok := httptest.NewRequest(http.MethodPost, protocol.ExecPath, strings.NewReader(body))
	ok.Header.Set("Authorization", protocol.AuthorizationValue("secret"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, ok)
	if rec.Code != http.StatusOK {
		t.Fatalf("right token %d %s", rec.Code, rec.Body.String())
	}
	var reply protocol.ExecReply
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil || !reply.OK {
		t.Fatalf("reply %s", rec.Body.String())
	}
}

func TestEmptyKeyNeverAuthorizes(t *testing.T) {
	handler := Handler("", func(context.Context, protocol.CommandCall) (string, error) {
		t.Fatal("empty key must not execute")
		return "", nil
	})
	req := httptest.NewRequest(http.MethodPost, protocol.ExecPath, strings.NewReader(`{"commandId":"c1","action":"host.collect.now"}`))
	req.Header.Set("Authorization", protocol.AuthorizationValue("secret"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty key %d", rec.Code)
	}
}

func TestAdvertiseBoundPort(t *testing.T) {
	if got := Advertise("127.0.0.1:7700"); got != "127.0.0.1:7700" {
		t.Fatalf("loopback %s", got)
	}
	if got := Advertise("192.168.1.8:7700"); got != "192.168.1.8:7700" {
		t.Fatalf("keep %s", got)
	}
	if got := Advertise("0.0.0.0:0"); got != "" {
		t.Fatalf("port 0 %s", got)
	}
	got := Advertise("0.0.0.0:7700")
	host, port, err := net.SplitHostPort(got)
	ip := net.ParseIP(host)
	if err != nil || port != "7700" || ip == nil || ip.IsUnspecified() {
		t.Fatalf("wildcard %s", got)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got = Advertise(ln.Addr().String())
	_, port, err = net.SplitHostPort(got)
	if err != nil || port == "" || port == "0" {
		t.Fatalf("bound %s", got)
	}
}
