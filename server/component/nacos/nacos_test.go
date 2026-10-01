package nacos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fluxguard/server/component/conn"
)

func TestParseProm(t *testing.T) {
	cfg, svc := ParseProm("# comment\nnacos_config_count 4\nnacos_monitor{name=\"serviceCount\"} 2\n")
	if cfg != 4 || svc != 2 {
		t.Fatalf("%v %v", cfg, svc)
	}
}

func TestCollectHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/readiness"):
			_, _ = w.Write([]byte(`{"status":"UP"}`))
		case strings.Contains(r.URL.Path, "prometheus"):
			_, _ = w.Write([]byte("nacos_config_count 6\nnacos_service_count 1\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	sample, status := collect(context.Background(), conn.Open{Address: srv.URL})
	if !status.Up || sample.ConfigCount != 6 || sample.ServiceCount != 1 {
		t.Fatalf("%+v %+v", sample, status)
	}
}
