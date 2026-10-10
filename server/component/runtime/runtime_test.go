package runtime

import (
	"context"
	"testing"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	dbx "github.com/fluxsce/dbx/db"
)

type fakeRunner struct{}

func (fakeRunner) Probe(context.Context, *dbx.DB, conn.Open) (models.Status, error) {
	return models.Status{}, nil
}

func (fakeRunner) Series(context.Context, *dbx.DB, string, string, int) ([]models.Point, error) {
	return nil, nil
}

func TestExecWithoutCommander(t *testing.T) {
	prev := Replace(catalog.ZK, fakeRunner{})
	t.Cleanup(func() { Replace(catalog.ZK, prev) })
	_, err := Exec(context.Background(), conn.Open{KindCode: catalog.ZK}, "mntr", nil)
	if err == nil || err.Error() != "这种组件没有这项操作" {
		t.Fatal(err)
	}
}
