package agent

import "testing"

func TestIsSystemName(t *testing.T) {
	cases := []struct {
		name, code string
		want       bool
	}{
		{"", "host", true},
		{"host", "host", true},
		{"host-a1b2c3d4", "host", true},
		{"appside-order@bo", "appside", true},
		{"go-order@bo", "go", true},
		{"本机主机", "host", false},
		{"host@box", "host", false},
		{"appside@box", "appside", false},
		{"host-a1b2c3d4", "appside", false},
		{"host", "appside", false},
	}
	for _, tc := range cases {
		if got := IsSystemName(tc.name, tc.code); got != tc.want {
			t.Fatalf("IsSystemName(%q,%q)=%v want %v", tc.name, tc.code, got, tc.want)
		}
	}
}

func TestResolveDisplayNameKeepsManual(t *testing.T) {
	got := ResolveDisplayName("机房A", "host", "本机主机", "box")
	if got != "机房A" {
		t.Fatalf("got %s", got)
	}
}

func TestResolveDisplayNameFillsSystemOnce(t *testing.T) {
	got := ResolveDisplayName("host-a1b2c3d4", "host", "本机主机", "box")
	if got != "本机主机" {
		t.Fatalf("preferred %s", got)
	}
	got = ResolveDisplayName("appside-a1b2c3d4", "appside", "", "box")
	if got != "appside@box" {
		t.Fatalf("host fallback %s", got)
	}
	got = ResolveDisplayName("go-order@bo", "go", "order-api", "box")
	if got != "order-api" {
		t.Fatalf("go preferred %s", got)
	}
}
