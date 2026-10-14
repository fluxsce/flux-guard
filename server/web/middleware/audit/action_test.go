package audit

import "testing"

func TestActionOf(t *testing.T) {
	cases := []struct {
		path, resource, want string
	}{
		{"/fluxguard/api/fg0021/queryUsers", "fg0021:search", ""},
		{"/fluxguard/api/fg0041/previewTemplate", "fg0041:view", ""},
		{"/fluxguard/api/auth/captcha", "", ""},
		{"/fluxguard/api/auth/login", "", ActionLogin},
		{"/fluxguard/api/auth/logout", "", ActionLogout},
		{"/fluxguard/api/auth/profile", "", ActionUpdate},
		{"/fluxguard/api/fg0021/createUser", "fg0021:add", ActionCreate},
		{"/fluxguard/api/fg0021/updateUser", "fg0021:edit", ActionUpdate},
		{"/fluxguard/api/fg0021/deleteUser", "fg0021:delete", ActionDelete},
		{"/fluxguard/api/fg0021/saveUserRoles", "fg0021:grant", ActionGrant},
		{"/fluxguard/api/fg0022/saveRoleResources", "fg0022:edit", ActionGrant},
		{"/fluxguard/api/fg0002/registerAgent", "fg0002:edit", ActionCreate},
		{"/fluxguard/api/fg0002/deleteAgents", "fg0002:delete", ActionDelete},
		{"/fluxguard/api/fg0002/dispatchCommand", "fg0002:command", ActionCommand},
		{"/fluxguard/api/fg0002/openShell", "fg0002:shell", ActionCommand},
		{"/fluxguard/api/fg0002/closeShell", "fg0002:shell", ActionCommand},
		{"/fluxguard/api/fg0002/shellInput", "fg0002:shell", ""},
		{"/fluxguard/api/fg0002/shellResize", "fg0002:shell", ""},
		{"/fluxguard/api/fg0002/queryCommands", "fg0002:view", ""},
		{"/fluxguard/api/fg0002/queryShellAudits", "fg0002:view", ""},
		{"/fluxguard/api/fg0024/saveSettings", "fg0024:edit", ActionUpdate},
		{"/fluxguard/api/fg0043/ackEvent", "fg0043:edit", ActionUpdate},
		{"/healthz", "", ""},
		{"/fluxguard/api/ingest/v1/batches", "", ""},
	}
	for _, item := range cases {
		if got := ActionOf(item.path, item.resource); got != item.want {
			t.Fatalf("%s %s got %q want %q", item.path, item.resource, got, item.want)
		}
	}
}

func TestNamed(t *testing.T) {
	if got := Named("CPU 告警", "T1"); got != "CPU 告警（T1）" {
		t.Fatalf("named %q", got)
	}
	if got := Named("admin", "admin"); got != "admin" {
		t.Fatalf("same %q", got)
	}
	if got := Named("", "ops"); got != "ops" {
		t.Fatalf("id %q", got)
	}
}

func TestModuleOf(t *testing.T) {
	if got := moduleOf("/fluxguard/api/fg0024/saveSettings", ""); got != "fg0024" {
		t.Fatalf("module %q", got)
	}
	if got := moduleOf("/fluxguard/api/auth/login", ""); got != "login" {
		t.Fatalf("login module %q", got)
	}
	if got := moduleOf("/fluxguard/api/fg0021/createUser", "fg0021:add"); got != "fg0021" {
		t.Fatalf("resource module %q", got)
	}
}
