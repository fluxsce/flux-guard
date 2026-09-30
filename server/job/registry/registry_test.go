package registry

import (
	"context"
	"testing"

	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
)

type stub struct{}

func (stub) Meta() contract.Meta {
	return contract.Meta{Code: "registry-stub", Title: "桩"}
}

func (stub) Run(context.Context, contract.Call) (contract.Result, error) {
	return contract.Result{}, nil
}

func TestCodes(t *testing.T) {
	Register(stub{})
	if !Has("registry-stub") {
		t.Fatal("missing")
	}
	found := false
	for _, code := range Codes() {
		if code == "registry-stub" {
			found = true
		}
	}
	if !found {
		t.Fatal(Codes())
	}
}

type kindStub struct{}

func (kindStub) Meta() contract.Meta {
	return contract.Meta{Code: "kind-stub", Title: "种类", Kind: "demo", Source: catalog.SourceBuiltin}
}

func (kindStub) Run(context.Context, contract.Call) (contract.Result, error) {
	return contract.Result{Data: []byte(`{"from":"kind"}`)}, nil
}

func TestBindAndUnregister(t *testing.T) {
	Register(kindStub{})
	if err := Bind(contract.Meta{Code: "demo-2", Title: "副本", Kind: "demo", Description: "按种类新建"}); err != nil {
		t.Fatal(err)
	}
	job, ok := Get("demo-2")
	if !ok || job.Meta().Source != catalog.SourceExternal || job.Meta().Title != "副本" {
		t.Fatalf("%#v", job.Meta())
	}
	if _, err := job.Run(context.Background(), contract.Call{}); err != nil {
		t.Fatal(err)
	}
	if err := Unregister("kind-stub"); err == nil {
		t.Fatal("builtin should stay")
	}
	if err := Unregister("demo-2"); err != nil {
		t.Fatal(err)
	}
	if Has("demo-2") {
		t.Fatal("external job still registered")
	}
	if _, ok := catalog.ByCode("demo-2"); ok {
		t.Fatal("catalog still has demo-2")
	}
}
