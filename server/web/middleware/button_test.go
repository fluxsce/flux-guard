package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"fluxguard/server/store"
	accessdao "fluxguard/server/web/middleware/dao"
	"fluxguard/server/web/request"
	login "fluxguard/server/web/views/login/controllers"
)

func TestRequireButtonAdminComesFromSession(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	prev := access
	t.Cleanup(func() { access = prev })
	UseAccess(accessdao.Open(db))

	if _, err := db.Exec(context.Background(), `
		DELETE FROM FG_AUTH_ROLE_RESOURCE
		WHERE roleId = 'ROLE_SUPER_ADMIN' AND resourceId = 'fg0002:edit'`, nil); err != nil {
		t.Fatal(err)
	}
	auth := login.Open(db, "default")
	adminTicket, err := auth.Login("admin", "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	viewerTicket, err := auth.Login("viewer", "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	adminSess, ok := auth.Principal(adminTicket.Token)
	if !ok || !adminSess.TenantAdmin {
		t.Fatalf("admin session %+v ok=%v", adminSess, ok)
	}
	viewerSess, ok := auth.Principal(viewerTicket.Token)
	if !ok || viewerSess.TenantAdmin {
		t.Fatalf("viewer session %+v ok=%v", viewerSess, ok)
	}
	if _, err := db.Exec(context.Background(), `
		UPDATE FG_USER SET tenantAdminFlag = 'N' WHERE userId = 'admin'`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `
		INSERT INTO FG_AUTH_USER_ROLE (
		  userRoleId, tenantId, userId, roleId, grantedBy, primaryRoleFlag,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  'UR_VIEWER_SA', 'default', 'viewer', 'ROLE_SUPER_ADMIN', 'system', 'N',
		  'system', 'system', 'INIT', 1, 'Y'
		)`, nil); err != nil {
		t.Fatal(err)
	}
	adminSess, ok = auth.Principal(adminTicket.Token)
	if !ok || !adminSess.TenantAdmin {
		t.Fatal("tenant admin flag must stay on the login session")
	}
	viewerSess, ok = auth.Principal(viewerTicket.Token)
	if !ok || viewerSess.TenantAdmin {
		t.Fatal("role grant must not mark a non-admin as tenant admin")
	}

	gin.SetMode(gin.TestMode)
	called := false
	handler := RequireButton("fg0002:edit", func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	adminRec := httptest.NewRecorder()
	adminCtx, _ := gin.CreateTestContext(adminRec)
	adminCtx.Request = httptest.NewRequest(http.MethodPost, "/edit?userId=viewer", strings.NewReader(`{"roleId":"ROLE_VIEWER"}`))
	adminCtx.Request = adminCtx.Request.WithContext(request.Attach(adminCtx.Request.Context(), adminSess))
	handler(adminCtx)
	if adminCtx.Writer.Status() != http.StatusNoContent || !called {
		t.Fatalf("admin status %d called %v", adminCtx.Writer.Status(), called)
	}

	called = false
	viewerRec := httptest.NewRecorder()
	viewerCtx, _ := gin.CreateTestContext(viewerRec)
	viewerCtx.Request = httptest.NewRequest(http.MethodPost, "/edit?userId=admin&roleId=ROLE_SUPER_ADMIN", strings.NewReader(`{"userId":"admin","roleId":"ROLE_SUPER_ADMIN"}`))
	viewerCtx.Request = viewerCtx.Request.WithContext(request.Attach(viewerCtx.Request.Context(), viewerSess))
	handler(viewerCtx)
	if viewerCtx.Writer.Status() != http.StatusForbidden || called {
		t.Fatalf("viewer status %d called %v", viewerCtx.Writer.Status(), called)
	}
}
