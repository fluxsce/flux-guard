package template

import "testing"

func TestRenderKeepsUnknownPlaceholder(t *testing.T) {
	out := Render("{{hostName}} {{value}} {{missing}}", map[string]string{"hostName": "web-01", "value": "95"})
	if out != "web-01 95 {{missing}}" {
		t.Fatalf("渲染结果 %q", out)
	}
}
