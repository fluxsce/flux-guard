package catalog

import "testing"

func TestRegister(t *testing.T) {
	Register(Spec{Code: "demo-job", Title: "演示", Source: SourceExternal, Scope: ScopeSystem})
	s, ok := ByCode("demo-job")
	if !ok || s.Source != SourceExternal || s.Scope != ScopeSystem {
		t.Fatalf("%#v", s)
	}
}
