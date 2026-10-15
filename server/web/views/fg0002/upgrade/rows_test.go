package upgrade

import (
	"context"
	"testing"

	"fluxguard/server/store"
	fgmodels "fluxguard/server/web/views/fg0002/models"

	dbx "github.com/fluxsce/dbx/db"
)

func TestInsertGetUpdateRelease(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	row := fgmodels.AgentRelease{
		ReleaseID: "r1", TenantID: "default", AgentCode: "host", VersionText: "1.0.0",
		SHA256: "abc", FileName: "pack.tar.gz",
		AddTime: "2026-10-15 01:00:00", AddWho: "admin",
		EditTime: "2026-10-15 01:00:00", EditWho: "admin",
		OprSeqFlag: "AGT", CurrentVersion: 1, ActiveFlag: "Y",
	}
	if err := db.Insert(ctx, tableRelease, &row); err != nil {
		t.Fatal(err)
	}

	got, err := loadRelease(ctx, db, "default", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if got.GoOS != "" || got.ExtProperty != "" || got.FileName != "pack.tar.gz" {
		t.Fatalf("got %+v", got)
	}

	got.ExtProperty = `{"pendingAgentIds":["a1"],"batchSize":8}`
	got.EditWho = "ops"
	got.CurrentVersion++
	pk, err := db.PK(&got)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, tableRelease, &got, pk); err != nil {
		t.Fatal(err)
	}

	again, err := loadRelease(ctx, db, "default", "r1")
	if err != nil {
		t.Fatal(err)
	}
	ext := applyReleaseMeta(&again)
	if again.CurrentVersion != 2 || again.EditWho != "ops" || len(ext.PendingAgentIDs) != 1 || ext.PendingAgentIDs[0] != "a1" {
		t.Fatalf("again %+v ext %+v", again, ext)
	}

	item := fgmodels.AgentReleaseItem{
		CommandID: "c1", TenantID: "default", ReleaseID: "r1", AgentID: "a1",
		AddTime: again.AddTime, AddWho: "admin", EditTime: again.EditTime, EditWho: "admin",
		OprSeqFlag: "AGT", CurrentVersion: 1, ActiveFlag: "Y",
	}
	if err := db.Insert(ctx, tableItem, &item); err != nil {
		t.Fatal(err)
	}
}

func TestGetReleaseNullText(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := db.Exec(ctx, `
		INSERT INTO FG_AGENT_RELEASE (
		  releaseId, tenantId, agentCode, versionText, sha256, fileName, goos, goarch,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, extProperty
		) VALUES (
		  'r2', 'default', 'host', '1.0.0', 'abc', 'pack.tar.gz', NULL, NULL,
		  '2026-10-15 01:00:00', 'admin', '2026-10-15 01:00:00', 'admin', 'AGT', 1, 'Y', NULL
		)`, nil); err != nil {
		t.Fatal(err)
	}
	var row fgmodels.AgentRelease
	if err := db.Get(ctx, &row, `
		SELECT releaseId, agentCode, versionText, sha256, fileName, goos, goarch, addTime, addWho, extProperty
		FROM FG_AGENT_RELEASE
		WHERE tenantId = @tenantId AND releaseId = @releaseId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": "default", "releaseId": "r2"}); err != nil {
		t.Fatal(err)
	}
	if row.GoOS != "" || row.GoArch != "" || row.ExtProperty != "" {
		t.Fatalf("NULL should become empty string, got %+v", row)
	}
}
