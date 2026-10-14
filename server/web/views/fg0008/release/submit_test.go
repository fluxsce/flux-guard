package release

import (
	"testing"

	appmodels "fluxguard/server/web/views/fg0008/models"
)

func TestSplitInstBatch(t *testing.T) {
	in := []appmodels.Instance{{InstID: "a"}, {InstID: "b"}, {InstID: "c"}}
	first, rest := splitInstBatch(in, 1)
	if len(first) != 1 || first[0].InstID != "a" || len(rest) != 2 {
		t.Fatalf("canary %v %v", first, rest)
	}
	all, none := splitInstBatch(in, 10)
	if len(all) != 3 || none != nil {
		t.Fatalf("all %v %v", all, none)
	}
}

func TestPathsFromSteps(t *testing.T) {
	target, backup, ok := pathsFromSteps([]appmodels.ReleaseStep{
		{Name: "download", Status: "done"},
		{Name: "replace", Status: "done", TargetPath: "/a/order.war", BackupPath: "/bak/order-1.war"},
	})
	if !ok || target != "/a/order.war" || backup != "/bak/order-1.war" {
		t.Fatalf("got %v %q %q", ok, target, backup)
	}
	if _, _, ok := pathsFromSteps([]appmodels.ReleaseStep{{Name: "replace", Status: "failed", BackupPath: "/x"}}); ok {
		t.Fatal("failed step")
	}
}

func TestParseCanary(t *testing.T) {
	if ParseCanary("") != 0 || ParseCanary("2") != 2 || ParseCanary("-1") != 0 {
		t.Fatal("parse")
	}
}
