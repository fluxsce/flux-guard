package conn

import (
	"context"
	"testing"

	"fluxguard/server/component/models"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
)

func TestSaveKeepsSecret(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx = request.With(ctx, request.Principal{TenantID: "default", UserID: "tester"})
	saved, err := Save(ctx, db, models.Input{
		KindCode: "redis", ComponentName: "本地", Address: "127.0.0.1:6379", Secret: "secret-1", IntervalSec: 30,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SecretSet != "Y" {
		t.Fatalf("%+v", saved)
	}
	open, err := LoadOpen(ctx, db, "default", saved.ComponentID)
	if err != nil || open.Secret != "secret-1" {
		t.Fatalf("%+v %v", open, err)
	}
	again, err := Save(ctx, db, models.Input{
		ComponentID: saved.ComponentID, KindCode: "redis", ComponentName: "本地", Address: "127.0.0.1:6379", IntervalSec: 45,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	open, err = LoadOpen(ctx, db, "default", again.ComponentID)
	if err != nil || open.Secret != "secret-1" || open.IntervalSec != 45 {
		t.Fatalf("%+v %v", open, err)
	}
}

func TestDraftUsesStoredSecret(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx = request.With(ctx, request.Principal{TenantID: "default", UserID: "tester"})
	saved, err := Save(ctx, db, models.Input{
		KindCode: "redis", ComponentName: "本地", Address: "127.0.0.1:6379", Secret: "secret-1", IntervalSec: 30,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := Draft(ctx, db, models.Input{
		ComponentID: saved.ComponentID, KindCode: "redis", Address: "10.0.0.8:6379",
		OptionText: "standalone:password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Secret != "secret-1" || draft.Address != "10.0.0.8:6379" || draft.OptionText != "standalone:password" {
		t.Fatalf("%+v", draft)
	}
	item, err := Get(ctx, db, "default", saved.ComponentID)
	if err != nil || item.Address != "127.0.0.1:6379" || item.UpFlag != "N" {
		t.Fatalf("%+v %v", item, err)
	}
	cleared, err := Draft(ctx, db, models.Input{
		KindCode: "redis", Address: "127.0.0.1:6379", OptionText: "standalone:none", Secret: "typed",
	})
	if err != nil || cleared.Secret != "" || cleared.OptionText != "standalone:none" {
		t.Fatalf("%+v %v", cleared, err)
	}
}
