package service

import (
	"context"
	"testing"

	"fluxguard/protocol"
	"fluxguard/server/store"
)

func TestAppsideUpsertKeepsDisplayName(t *testing.T) {
	svc := testService(t)
	batch := protocol.Batch{
		TenantID: "default", AgentID: "side-1", BatchID: "batch-app-side-1", AgentCode: protocol.AgentAppside,
		AppSide: &protocol.AppSide{
			InstanceName: "tomcat-order", RuntimeKind: "systemd", Mode: "tomcat",
			RunningFlag: "Y", MachineID: "machine-1",
			Artifacts: []protocol.AppArtifact{{AppName: "order", Kind: "war", Version: "1"}},
		},
	}
	if _, err := svc.Accept(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.db.Exec(context.Background(), `
		UPDATE FG_APP SET appName = '订单' WHERE tenantId = 'default' AND appCode = 'order'`, nil); err != nil {
		t.Fatal(err)
	}
	batch.BatchID = "batch-app-side-2"
	batch.AppSide.RunningFlag = "N"
	batch.AppSide.Artifacts[0].Version = "2"
	if _, err := svc.Accept(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	var name, version, running string
	if err := store.Scan(context.Background(), svc.db, `
		SELECT a.appName, COALESCE(i.versionText, ''), i.runningFlag
		FROM FG_APP a JOIN FG_APP_INST i ON i.tenantId = a.tenantId AND i.appCode = a.appCode
		WHERE a.tenantId = 'default' AND a.appCode = 'order'`, nil, &name, &version, &running); err != nil {
		t.Fatal(err)
	}
	if name != "订单" || version != "2" || running != "N" {
		t.Fatalf("name %s version %s running %s", name, version, running)
	}
}

func TestFilesHeartbeatKeepsSyncedChildren(t *testing.T) {
	svc := testService(t)
	full := protocol.Batch{
		TenantID: "default", AgentID: "files-1", BatchID: "batch-files-full", AgentCode: protocol.AgentAppside,
		AppSide: &protocol.AppSide{
			InstanceName: "war-repo", RuntimeKind: "command", Mode: "files", RunningFlag: "N",
			BasePath: "E:\\war",
			Artifacts: []protocol.AppArtifact{
				{AppName: "war-repo", Kind: "runtime", Path: "E:\\war"},
				{AppName: "demo", Kind: "war", Path: "E:\\war\\demo.war"},
			},
		},
	}
	if _, err := svc.Accept(context.Background(), full); err != nil {
		t.Fatal(err)
	}
	beat := protocol.Batch{
		TenantID: "default", AgentID: "files-1", BatchID: "batch-files-beat", AgentCode: protocol.AgentAppside,
		AppSide: &protocol.AppSide{
			InstanceName: "war-repo", RuntimeKind: "command", Mode: "files", RunningFlag: "N",
			BasePath: "E:\\war",
			Artifacts: []protocol.AppArtifact{
				{AppName: "war-repo", Kind: "runtime", Path: "E:\\war"},
			},
		},
	}
	if _, err := svc.Accept(context.Background(), beat); err != nil {
		t.Fatal(err)
	}
	var active int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_APP_INST
		WHERE tenantId = 'default' AND agentId = 'files-1' AND appCode = 'demo' AND activeFlag = 'Y'`, nil, &active); err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("synced war retired by heartbeat: %d", active)
	}
}

func TestAppsideReportFillsAgentRow(t *testing.T) {
	svc := testService(t)
	batch := protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-app-agent-row", AgentCode: protocol.AgentAppside,
		AppSide: &protocol.AppSide{
			InstanceName: "war-repo", RuntimeKind: "command", Mode: "files", RunningFlag: "N",
			HostName: "box-a", IPAddress: "10.0.0.8", CommandAddress: "10.0.0.8:7701",
			BasePath: "E:\\war\\repo",
			Artifacts: []protocol.AppArtifact{{AppName: "war-repo", Kind: "runtime", Path: "E:\\war\\repo"}},
		},
	}
	if _, err := svc.Accept(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	var host, ip, cmd string
	if err := store.Scan(context.Background(), svc.db, `
		SELECT COALESCE(hostName, ''), COALESCE(ipAddress, ''), COALESCE(commandAddress, '')
		FROM FG_AGENT WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &host, &ip, &cmd); err != nil {
		t.Fatal(err)
	}
	if host != "box-a" || ip != "10.0.0.8" || cmd != "10.0.0.8:7701" {
		t.Fatalf("agent row %s %s %s", host, ip, cmd)
	}
}

func TestOneAgentKeepsSameWarOnTwoTomcats(t *testing.T) {
	svc := testService(t)
	batch := protocol.Batch{
		TenantID: "default", AgentID: "side-box", BatchID: "batch-two-tomcat", AgentCode: protocol.AgentAppside,
		AppSides: []protocol.AppSide{
			{
				InstanceName: "tomcat-a", RuntimeKind: "systemd", Mode: "tomcat", RunningFlag: "Y",
				Artifacts: []protocol.AppArtifact{{AppName: "order", Kind: "war", Version: "1"}},
			},
			{
				InstanceName: "tomcat-b", RuntimeKind: "systemd", Mode: "tomcat", RunningFlag: "Y",
				Artifacts: []protocol.AppArtifact{{AppName: "order", Kind: "war", Version: "9"}},
			},
		},
	}
	if _, err := svc.Accept(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_APP_INST
		WHERE tenantId = 'default' AND agentId = 'side-box' AND appCode = 'order' AND activeFlag = 'Y'`, nil, &count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("instances %d", count)
	}
}
