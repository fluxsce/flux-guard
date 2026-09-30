package init

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"fluxguard/server/setting"
	"fluxguard/server/store"
	login "fluxguard/server/web/views/login/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

func TestAuditLedger(t *testing.T) {
	authDB, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if err := setting.Load(context.Background(), authDB); err != nil {
		t.Fatal(err)
	}
	login.Open(authDB, "default")
	handler := New(authDB)

	admin := signIn(t, handler, "admin", "fluxguard")
	if n := auditCount(t, authDB, "LOGIN", "Y"); n != 1 {
		t.Fatalf("login rows %d", n)
	}

	before := auditTotal(t, authDB)
	queried := call(t, handler, http.MethodGet, "/fluxguard/api/fg0021/queryUsers", admin, nil)
	if queried.Code != http.StatusOK {
		t.Fatalf("query %d %s", queried.Code, queried.Body.String())
	}
	captcha := call(t, handler, http.MethodPost, "/fluxguard/api/auth/captcha", "", map[string]string{"type": "random"})
	if captcha.Code != http.StatusOK {
		t.Fatalf("captcha %d %s", captcha.Code, captcha.Body.String())
	}
	if auditTotal(t, authDB) != before {
		t.Fatal("query and captcha should stay out of the audit ledger")
	}

	bad := call(t, handler, http.MethodPost, "/fluxguard/api/auth/login", "", map[string]string{
		"userId": "admin", "password": "not-the-password",
	})
	if bad.Code == http.StatusOK {
		t.Fatal("bad password accepted")
	}
	if n := auditCount(t, authDB, "LOGIN_FAIL", "N"); n != 1 {
		t.Fatalf("login fail rows %d", n)
	}
	var detail string
	var failReason string
	if err := store.Scan(context.Background(), authDB, `
		SELECT COALESCE(detailText, ''), COALESCE(failReason, '') FROM FG_AUTH_AUDIT_LOG WHERE action = 'LOGIN_FAIL'`,
		nil, &detail, &failReason); err != nil {
		t.Fatal(err)
	}
	if detail != "登录" {
		t.Fatalf("login fail detail %q", detail)
	}
	if failReason == "" || strings.Contains(failReason, "not-the-password") {
		t.Fatalf("login fail reason %q", failReason)
	}

	viewer := signIn(t, handler, "viewer", "fluxguard")
	denied := call(t, handler, http.MethodPost, "/fluxguard/api/fg0024/saveSettings", viewer, map[string]int{
		"requestTimeoutSeconds": 60, "sessionExpireHours": 8, "defaultPageSize": 20, "maxPageSize": 100,
	})
	if denied.Code != http.StatusForbidden {
		t.Fatalf("viewer save %d", denied.Code)
	}
	var module, resource, result string
	if err := store.Scan(context.Background(), authDB, `
		SELECT moduleCode, resourceCode, resultFlag FROM FG_AUTH_AUDIT_LOG
		WHERE action = 'UPDATE' AND userId = 'viewer'`, nil, &module, &resource, &result); err != nil {
		t.Fatal(err)
	}
	if module != "fg0024" || resource != "fg0024:edit" || result != "N" {
		t.Fatalf("denied audit module=%s resource=%s result=%s", module, resource, result)
	}

	saved := call(t, handler, http.MethodPost, "/fluxguard/api/fg0024/saveSettings", admin, map[string]int{
		"requestTimeoutSeconds": 60, "sessionExpireHours": 8, "defaultPageSize": 20, "maxPageSize": 100,
	})
	if saved.Code != http.StatusOK {
		t.Fatalf("admin save %d %s", saved.Code, saved.Body.String())
	}
	if n := auditCount(t, authDB, "UPDATE", "Y"); n != 1 {
		t.Fatalf("update rows %d", n)
	}

	out := call(t, handler, http.MethodPost, "/fluxguard/api/auth/logout", admin, nil)
	if out.Code != http.StatusOK {
		t.Fatalf("logout %d %s", out.Code, out.Body.String())
	}
	if n := auditCount(t, authDB, "LOGOUT", "Y"); n != 1 {
		t.Fatalf("logout rows %d", n)
	}
}

func auditCount(t *testing.T, db *dbx.DB, action, result string) int {
	t.Helper()
	var n int
	err := store.Scan(context.Background(), db, `
		SELECT count(*) FROM FG_AUTH_AUDIT_LOG WHERE action = @action AND resultFlag = @result`,
		dbx.Args{"action": action, "result": result}, &n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func auditTotal(t *testing.T, db *dbx.DB) int {
	t.Helper()
	var n int
	if err := store.Scan(context.Background(), db, `SELECT count(*) FROM FG_AUTH_AUDIT_LOG`, nil, &n); err != nil {
		t.Fatal(err)
	}
	return n
}
