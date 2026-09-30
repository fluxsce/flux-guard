package dao

import (
	"context"
	"testing"

	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0023/models"
)

func TestListBuildsResourceCatalog(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	dao := NewResourceDAO(db)
	rows, err := dao.List(context.Background(), "default", Filter{})
	if err != nil {
		t.Fatal(err)
	}
	tree := models.Tree(rows)
	var system *models.Node
	for _, root := range tree {
		if root.ResourceCode == "group:system" {
			system = root
		}
	}
	if system == nil {
		t.Fatal("missing system group")
	}
	var catalog *models.Node
	for _, child := range system.Children {
		if child.ResourceCode == "fg0023" {
			catalog = child
		}
	}
	if catalog == nil || catalog.ResourcePath != "/fg0023" {
		t.Fatal("resource catalog module missing")
	}
	if catalog.AddWho != "system" || catalog.EditWho != "system" || catalog.AddTime == "" || catalog.EditTime == "" {
		t.Fatal("audit fields missing")
	}
	buttons := map[string]bool{}
	for _, child := range catalog.Children {
		buttons[child.ResourceCode] = true
	}
	if len(catalog.Children) != 2 || !buttons["fg0023:search"] || !buttons["fg0023:view"] {
		t.Fatal("search or view button missing")
	}
	found, err := dao.Get(context.Background(), "default", catalog.ResourceID)
	if err != nil || found.ResourceCode != "fg0023" || found.ResourcePath != "/fg0023" {
		t.Fatalf("get resource %v %v", found, err)
	}
	filtered, err := dao.List(context.Background(), "default", Filter{ResourceCode: "fg0023:view"})
	if err != nil {
		t.Fatal(err)
	}
	narrow := models.Tree(filtered)
	if len(narrow) != 1 || narrow[0].ResourceCode != "group:system" {
		t.Fatal("filtered tree lost the parent group")
	}
	module := narrow[0].Children
	if len(module) != 1 || module[0].ResourceCode != "fg0023" || len(module[0].Children) != 1 || module[0].Children[0].ResourceCode != "fg0023:view" {
		t.Fatal("filtered tree missing the view button")
	}
}
