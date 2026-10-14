package controllers

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"fluxguard/server/utils/secret"
	"fluxguard/server/setting"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestSQLSeedContainsPasswordAndResources(t *testing.T) {
	userSQL, err := os.ReadFile("../../../../../scripts/db/mysql/FG_USER.sql")
	if err != nil {
		t.Fatal(err)
	}
	hash := bcryptIn(string(userSQL))
	if hash == "" || !secret.MatchPassword(hash, "fluxguard") {
		t.Fatal("FG_USER.sql password hash drifted")
	}
	agentSQL, err := os.ReadFile("../../../../../scripts/db/mysql/FG_AGENT.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agentSQL), secret.HashToken("agt_dev_local")) {
		t.Fatal("FG_AGENT.sql token hash drifted")
	}
	resourceSQL, err := os.ReadFile("../../../../../scripts/db/mysql/FG_AUTH_RESOURCE.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(resourceSQL)
	for _, code := range []string{"fg0001:view", "fg0002:edit", "fg0022:view", "fg0022:edit", "fg0023:view", "fg0024:view", "fg0024:edit", "fg0008", "fg0008:release"} {
		if !strings.Contains(text, "'"+code+"'") {
			t.Errorf("SQL missing %s", code)
		}
	}
}

func testAuth(t *testing.T) *Auth {
	t.Helper()
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return Open(db, "default")
}

func TestLoginRejectsExpiredUser(t *testing.T) {
	auth := testAuth(t)
	_, err := auth.DB().Exec(context.Background(),
		`UPDATE FG_USER SET userExpireDate = @t WHERE userId = @userId`,
		dbx.Args{"t": store.FormatTime(time.Now().Add(-time.Hour)), "userId": "viewer"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Login("viewer", "fluxguard"); !errors.Is(err, ErrExpired) {
		t.Fatalf("got %v", err)
	}
}

func TestLoginCachesSessionForSettingTTL(t *testing.T) {
	auth := testAuth(t)
	ticket, err := auth.Login("viewer", "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	ttl := setting.SessionTTL("default")
	remain := time.Until(ticket.Expire)
	if remain < ttl-time.Minute || remain > ttl+time.Minute {
		t.Fatalf("session ttl %s, want about %s", remain, ttl)
	}
	sess, ok := auth.Principal(ticket.Token)
	if !ok || sess.UserID != "viewer" || sess.RealName == "" {
		t.Fatalf("cached session %+v ok=%v", sess, ok)
	}
}

func TestExpiredSessionsAreDropped(t *testing.T) {
	auth := testAuth(t)
	if err := saveSession(session{Token: "old", Until: time.Now().Add(20 * time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, ok := auth.Principal("missing"); ok {
		t.Fatal("missing session")
	}
	if _, ok := loadSession("old"); ok {
		t.Fatal("expired session still stored")
	}
}

func bcryptIn(sql string) string {
	const mark = "$2a$"
	start := strings.Index(sql, mark)
	if start < 0 {
		return ""
	}
	end := start + len(mark)
	for end < len(sql) && sql[end] != '\'' && sql[end] != '"' && sql[end] != ' ' {
		end++
	}
	return sql[start:end]
}
