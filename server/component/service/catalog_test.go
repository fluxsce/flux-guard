package service

import (
	"context"
	"testing"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/web/request"
)

func TestCatalogReleasesKindRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx = request.With(ctx, request.Principal{UserID: "tester", TenantID: "default"})
	kinds, err := Catalog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) == 0 {
		t.Fatal("expected kinds")
	}
	for _, kind := range kinds {
		if len(kind.Panels) == 0 {
			t.Fatalf("%s panels missing", kind.KindCode)
		}
	}
}
