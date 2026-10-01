package init

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fluxguard/common/buildinfo"
	"fluxguard/common/logger"
	"fluxguard/protocol"
	"fluxguard/server/setting"
	"fluxguard/server/store"
	login "fluxguard/server/web/views/login/controllers"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	t.Cleanup(func() { logger.SetOutput(os.Stderr) })
	return &buf
}

func TestAccessLogSkipsQuery(t *testing.T) {
	buf := captureLog(t)
	handler := New(nil)
	res := call(t, handler, http.MethodGet, "/healthz?token=secret-token-value", "", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("healthz %d", res.Code)
	}
	line := buf.String()
	begin := findAccess(t, line, "请求开始", "/healthz")
	finish := findAccess(t, line, "请求完成", "/healthz")
	if begin["method"] != "GET" || finish["method"] != "GET" || finish["status"] != float64(200) {
		t.Fatalf("access log %q", line)
	}
	if _, ok := begin["status"]; ok {
		t.Fatalf("start log has status %q", line)
	}
	for _, key := range []string{"path", "client_ip", "trace_id", "duration_ms", "response_size"} {
		if key == "duration_ms" || key == "response_size" {
			if _, ok := finish[key]; !ok {
				t.Fatalf("missing %s in %q", key, line)
			}
			continue
		}
		if finish[key] == nil || finish[key] == "" {
			t.Fatalf("missing %s in %q", key, line)
		}
	}
	if begin["msg"] != "请求开始" || finish["msg"] != "请求完成" {
		t.Fatalf("fields still inside msg %q", line)
	}
	if strings.Contains(line, "secret-token-value") {
		t.Fatalf("access log %q", line)
	}
	if countAccess(line, "请求完成", "/healthz") != 1 {
		t.Fatalf("access log %q", line)
	}
	if res.Header().Get("X-Trace-ID") == "" {
		t.Fatal("missing trace id")
	}
}

func TestStaticAccessLogOnce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("console"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := captureLog(t)
	handler := Static(dir, New(nil))

	page := call(t, handler, http.MethodGet, "/fluxguard/", "", nil)
	if page.Code != http.StatusOK || !bytes.Contains(page.Body.Bytes(), []byte("console")) {
		t.Fatalf("page %d %s", page.Code, page.Body.String())
	}
	pageLog := findAccess(t, buf.String(), "请求完成", "/fluxguard/")
	if pageLog["status"] != float64(200) {
		t.Fatalf("page log %q", buf.String())
	}

	root := call(t, handler, http.MethodGet, "/", "", nil)
	if root.Code != http.StatusFound {
		t.Fatalf("redirect %d", root.Code)
	}
	rootLog := findAccess(t, buf.String(), "请求完成", "/")
	if rootLog["status"] != float64(302) {
		t.Fatalf("redirect log %q", buf.String())
	}

	buf.Reset()
	api := call(t, handler, http.MethodGet, "/healthz", "", nil)
	if api.Code != http.StatusOK {
		t.Fatalf("healthz %d", api.Code)
	}
	if countAccess(buf.String(), "请求完成", "/healthz") != 1 {
		t.Fatalf("api log %q", buf.String())
	}
}

func findAccess(t *testing.T, raw, msg, path string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(raw, "\n") {
		obj := parseLog(line)
		if obj == nil || obj["msg"] != msg || obj["path"] != path {
			continue
		}
		return obj
	}
	t.Fatalf("missing %s %s in %q", msg, path, raw)
	return nil
}

func countAccess(raw, msg, path string) int {
	n := 0
	for _, line := range strings.Split(raw, "\n") {
		obj := parseLog(line)
		if obj != nil && obj["msg"] == msg && obj["path"] == path {
			n++
		}
	}
	return n
}

func parseLog(line string) map[string]any {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil
	}
	return obj
}

func TestHealthzReportsBuildVersion(t *testing.T) {
	handler := New(nil)
	res := call(t, handler, http.MethodGet, "/healthz", "", nil)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"version":"`+buildinfo.Version+`"`)) {
		t.Fatalf("healthz %d %s", res.Code, res.Body.String())
	}
}

func TestModulesLoginAndViewerDenied(t *testing.T) {
	authDB, authCleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authCleanup)
	if err := setting.Load(context.Background(), authDB); err != nil {
		t.Fatal(err)
	}
	login.Open(authDB, "default")
	handler := New(authDB)

	admin := signIn(t, handler, "admin", "fluxguard")
	viewer := signIn(t, handler, "viewer", "fluxguard")

	res := call(t, handler, http.MethodGet, "/fluxguard/api/fg0021/queryUsers", viewer, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("viewer users %d %s", res.Code, res.Body.String())
	}
	res = call(t, handler, http.MethodGet, "/fluxguard/api/fg0024/querySettings", viewer, nil)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"requestTimeoutSeconds":120`)) {
		t.Fatalf("viewer settings %d %s", res.Code, res.Body.String())
	}
	res = call(t, handler, http.MethodPost, "/fluxguard/api/fg0024/saveSettings", viewer, map[string]int{
		"requestTimeoutSeconds": 60, "sessionExpireHours": 8, "defaultPageSize": 20, "maxPageSize": 100,
	})
	if res.Code != http.StatusForbidden {
		t.Fatalf("viewer save settings %d %s", res.Code, res.Body.String())
	}
	res = call(t, handler, http.MethodPost, "/fluxguard/api/fg0024/saveSettings", admin, map[string]int{
		"requestTimeoutSeconds": 60, "sessionExpireHours": 8, "defaultPageSize": 20, "maxPageSize": 100,
	})
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"currentVersion":1`)) {
		t.Fatalf("admin save settings %d %s", res.Code, res.Body.String())
	}
	res = call(t, handler, http.MethodGet, "/fluxguard/api/fg0023/queryResources", viewer, nil)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"resourceCode":"fg0023"`)) {
		t.Fatalf("viewer resources %d %s", res.Code, res.Body.String())
	}
	res = call(t, handler, http.MethodPost, "/fluxguard/api/fg0002/registerAgent", viewer, map[string]string{"agentName": "nope"})
	if res.Code != http.StatusForbidden {
		t.Fatalf("viewer register %d %s", res.Code, res.Body.String())
	}

	res = call(t, handler, http.MethodPost, protocol.IngestPath, "", map[string]any{
		"batchId": "batch-http-1", "agentCode": "host",
		"samples": []map[string]any{{"metricName": "runtime.cpu.count", "metricValue": 8}},
	})
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("missing token %d", res.Code)
	}
	req := httptest.NewRequest(http.MethodPost, protocol.IngestPath, bytes.NewReader(mustJSON(t, map[string]any{
		"batchId": "batch-http-1", "agentCode": "host",
		"samples": []map[string]any{{"metricName": "runtime.cpu.count", "metricValue": 8}},
	})))
	req.Header.Set("Authorization", "Bearer agt_dev_local")
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("ingest %d %s", res.Code, res.Body.String())
	}
	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, protocol.IngestPath, bytes.NewReader(mustJSON(t, map[string]any{
		"batchId": "batch-http-1", "agentCode": "host",
		"samples": []map[string]any{{"metricName": "runtime.cpu.count", "metricValue": 8}},
	})))
	req.Header.Set("Authorization", "Bearer agt_dev_local")
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(res, req)
	var wrapped struct {
		Data struct {
			Duplicate bool `json:"duplicate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &wrapped); err != nil {
		t.Fatal(err)
	}
	if !wrapped.Data.Duplicate {
		t.Fatalf("body=%s", res.Body.String())
	}

	agents := call(t, handler, http.MethodGet, "/fluxguard/api/fg0002/queryAgents", admin, nil)
	if !bytes.Contains(agents.Body.Bytes(), []byte(`"onlineFlag":"Y"`)) {
		t.Fatalf("agent should be online: %s", agents.Body.String())
	}
}

func TestSelfProfileCannotEscalate(t *testing.T) {
	authDB, authCleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authCleanup)
	login.Open(authDB, "default")
	handler := New(authDB)
	viewer := signIn(t, handler, "viewer", "fluxguard")
	other := signIn(t, handler, "viewer", "fluxguard")
	admin := signIn(t, handler, "admin", "fluxguard")

	denied := call(t, handler, http.MethodPost, "/fluxguard/api/auth/profile", viewer, map[string]any{
		"userId": "admin", "tenantId": "default", "userName": "admin",
		"tenantAdminFlag": "Y", "statusFlag": "N", "realName": "越权",
	})
	if denied.Code != http.StatusBadRequest || !bytes.Contains(denied.Body.Bytes(), []byte("只能修改本人")) {
		t.Fatalf("privilege fields %d %s", denied.Code, denied.Body.String())
	}

	saved := call(t, handler, http.MethodPost, "/fluxguard/api/auth/profile", viewer, map[string]any{
		"realName": "只读甲", "email": "viewer@example.com", "mobile": "13800000000", "gender": 1,
	})
	if saved.Code != http.StatusOK || !bytes.Contains(saved.Body.Bytes(), []byte(`"realName":"只读甲"`)) {
		t.Fatalf("save self %d %s", saved.Code, saved.Body.String())
	}
	var adminFlag, adminName string
	if err := store.Scan(context.Background(), authDB, `
		SELECT tenantAdminFlag, realName FROM FG_USER WHERE userId = 'admin'`, nil, &adminFlag, &adminName); err != nil {
		t.Fatal(err)
	}
	if adminFlag != "Y" || adminName != "系统管理员" {
		t.Fatalf("admin changed flag=%s name=%s", adminFlag, adminName)
	}
	var viewerFlag string
	if err := store.Scan(context.Background(), authDB, `
		SELECT tenantAdminFlag FROM FG_USER WHERE userId = 'viewer'`, nil, &viewerFlag); err != nil {
		t.Fatal(err)
	}
	if viewerFlag != "N" {
		t.Fatalf("viewer admin flag %s", viewerFlag)
	}

	badPwd := call(t, handler, http.MethodPost, "/fluxguard/api/auth/profile", viewer, map[string]any{
		"realName": "只读甲", "currentPassword": "wrong-pass", "newPassword": "new-pass-1",
	})
	if badPwd.Code != http.StatusBadRequest {
		t.Fatalf("bad password %d %s", badPwd.Code, badPwd.Body.String())
	}
	changed := call(t, handler, http.MethodPost, "/fluxguard/api/auth/profile", viewer, map[string]any{
		"realName": "只读甲", "currentPassword": "fluxguard", "newPassword": "new-pass-1",
	})
	if changed.Code != http.StatusOK {
		t.Fatalf("change password %d %s", changed.Code, changed.Body.String())
	}
	if call(t, handler, http.MethodGet, "/fluxguard/api/auth/me", other, nil).Code != http.StatusUnauthorized {
		t.Fatal("old session should be dropped after password change")
	}
	signIn(t, handler, "viewer", "new-pass-1")
	old := call(t, handler, http.MethodPost, "/fluxguard/api/auth/login", "", map[string]string{
		"userId": "viewer", "password": "fluxguard",
	})
	if old.Code == http.StatusOK {
		t.Fatal("old password still works")
	}
	adminMe := call(t, handler, http.MethodGet, "/fluxguard/api/auth/me", admin, nil)
	if adminMe.Code != http.StatusOK || !bytes.Contains(adminMe.Body.Bytes(), []byte(`"realName":"系统管理员"`)) {
		t.Fatalf("admin session %d %s", adminMe.Code, adminMe.Body.String())
	}
}

func TestHostMachinesStaySeparate(t *testing.T) {
	authDB, authCleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authCleanup)
	if err := setting.Load(context.Background(), authDB); err != nil {
		t.Fatal(err)
	}
	login.Open(authDB, "default")
	handler := New(authDB)
	admin := signIn(t, handler, "admin", "fluxguard")
	for _, item := range []struct{ batch, machine string }{
		{"batch-m-aaa11111", "machine-aaaa1111"},
		{"batch-m-bbb22222", "machine-bbbb2222"},
	} {
		res := call(t, handler, http.MethodPost, protocol.IngestPath, "agt_dev_local", map[string]any{
			"batchId": item.batch, "agentCode": "host",
			"samples": []map[string]any{{"metricName": "host.cpu.count", "metricValue": 2}},
			"host": map[string]any{
				"hostName": "box", "osName": "linux", "archName": "amd64",
				"cpuCount": 2, "memTotalBytes": 1024, "machineId": item.machine,
			},
		})
		if res.Code != http.StatusOK {
			t.Fatalf("ingest %s %d %s", item.machine, res.Code, res.Body.String())
		}
	}
	agents := call(t, handler, http.MethodGet, "/fluxguard/api/fg0002/queryAgents", admin, nil)
	body := agents.Body.String()
	if !bytes.Contains(agents.Body.Bytes(), []byte("machine-aaaa1111")) || !bytes.Contains(agents.Body.Bytes(), []byte("machine-bbbb2222")) {
		t.Fatalf("agents=%s", body)
	}
}

func signIn(t *testing.T, handler http.Handler, user, password string) string {
	t.Helper()
	res := call(t, handler, http.MethodPost, "/fluxguard/api/auth/login", "", map[string]string{
		"userId": user, "password": password,
	})
	if res.Code != http.StatusOK {
		t.Fatalf("login %s %d %s", user, res.Code, res.Body.String())
	}
	var wrapped struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &wrapped); err != nil {
		t.Fatal(err)
	}
	if wrapped.Data.Token == "" {
		t.Fatal("empty token")
	}
	return wrapped.Data.Token
}

func TestVersionWithoutSession(t *testing.T) {
	handler := New(nil)
	res := call(t, handler, http.MethodGet, "/fluxguard/api/version", "", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("version %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"version"`) {
		t.Fatal(res.Body.String())
	}
}

func call(t *testing.T, handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
