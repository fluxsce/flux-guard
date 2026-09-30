package dao

import (
	"context"
	"testing"

	"fluxguard/server/setting"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
)

func adminCtx() context.Context {
	return request.With(context.Background(), request.Principal{TenantID: "default", UserID: "admin"})
}

func TestSaveWebTimeout(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	access := setting.WebTimeout{
		RequestTimeoutSeconds: 90,
		SessionExpireHours:    8,
		DefaultPageSize:       20,
		MaxPageSize:           100,
	}
	content, err := setting.Encode(access)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewSettingDAO(db)
	version, err := repo.Save(adminCtx(), setting.GroupWebTimeout, 0, content)
	if err != nil || version != 1 {
		t.Fatalf("save %v version=%d", err, version)
	}
	if err := setting.Load(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	got := setting.Get("default")
	if got.RequestTimeoutSeconds != 90 || got.SessionExpireHours != 8 || got.DefaultPageSize != 20 || got.MaxPageSize != 100 {
		t.Fatalf("cache %+v", got)
	}
	if _, err := repo.Save(adminCtx(), setting.GroupWebTimeout, 0, content); err != ErrConflict {
		t.Fatalf("stale version: %v", err)
	}
}
