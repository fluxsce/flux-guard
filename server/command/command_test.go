package command

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentcommand "fluxguard/common/command"
	"fluxguard/protocol"
	"fluxguard/server/command/models"
	"fluxguard/server/helper/agent"
	"fluxguard/server/setting"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
	"github.com/gin-gonic/gin"
)

func TestDispatchPushAndPull(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if err := setting.Load(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	svc, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	agents := agent.Open(db)
	agentID, err := agents.EnsureMachineAgent("default", "host", "machine-1")
	if err != nil {
		t.Fatal(err)
	}
	agent := httptest.NewServer(agentcommand.Handler(setting.DevIngestPlain, func(context.Context, protocol.CommandCall) (protocol.CommandOutcome, error) {
		return protocol.CommandOutcome{Message: "已触发采集"}, nil
	}))
	t.Cleanup(agent.Close)
	insertHost(t, db, agentID, strings.TrimPrefix(agent.URL, "http://"), "127.0.0.1")

	_, err = svc.Dispatch(context.Background(), "default", "admin", DispatchInput{AgentID: agentID, Action: "rm -rf"})
	if err == nil {
		t.Fatal("unknown action accepted")
	}

	pushed, err := svc.Dispatch(context.Background(), "default", "admin", DispatchInput{
		AgentID: agentID, Action: protocol.ActionCollectNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.Status != "done" || pushed.DeliverMode != models.DeliverPush || pushed.Message != "已触发采集" {
		t.Fatalf("push %#v", pushed)
	}

	insertHost(t, db, agentID, "203.0.113.9:7700", "10.1.2.3")
	waiting, err := svc.Dispatch(context.Background(), "default", "admin", DispatchInput{
		AgentID: agentID, Action: protocol.ActionIntervalSet, IntervalSec: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Status != "waiting" {
		t.Fatalf("pull fallback %#v", waiting)
	}

	router := gin.New()
	Register(router, agents, svc)
	api := httptest.NewServer(router)
	t.Cleanup(api.Close)
	call := waitCommand(t, api.URL, "machine-1")
	if call.CommandID != waiting.CommandID || call.IntervalSec != 30 {
		t.Fatalf("wait %#v", call)
	}
	postCommandResult(t, api.URL, "machine-1", protocol.CommandResult{
		CommandID: call.CommandID, OK: true, Message: "间隔已改",
		Steps: []protocol.CommandStep{{
			Name: "startup", Status: "failed",
			Detail: "重启后进程未运行，且日志未见启动完成\n--- catalina.out ---\nAddress already in use",
		}},
	})
	done, err := svc.Get(context.Background(), "default", call.CommandID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "done" || done.DeliverMode != models.DeliverPull || done.Message != "间隔已改" {
		t.Fatalf("pull result %#v", done)
	}
	if len(done.Steps) != 1 || !strings.Contains(done.Steps[0].Detail, "catalina.out") {
		t.Fatalf("steps %#v", done.Steps)
	}
}

func insertHost(t *testing.T, db *dbx.DB, agentID, addr, ip string) {
	t.Helper()
	now := store.FormatTime(time.Now().UTC())
	_, err := db.Exec(context.Background(), `
		INSERT INTO FG_AGENT_HOST (
		  agentId, tenantId, hostName, osName, archName, ipAddress, ipList, commandAddress,
		  reportedTime, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, 'default', 'box', 'linux', 'amd64', @ip, @ip, @addr,
		  @now, 'system', 'system', 'TEST', 1, 'Y'
		)
		ON CONFLICT(tenantId, agentId) DO UPDATE SET
		  ipAddress = excluded.ipAddress, ipList = excluded.ipList, commandAddress = excluded.commandAddress`,
		dbx.Args{"agentId": agentID, "ip": ip, "addr": addr, "now": now})
	if err != nil {
		t.Fatal(err)
	}
}

func waitCommand(t *testing.T, serverURL, machineID string) protocol.CommandCall {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, serverURL+protocol.WaitPath+"?agentCode=host&machineId="+machineID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(setting.DevIngestPlain))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("wait %d %s", res.StatusCode, body)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Command *protocol.CommandCall `json:"command"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || !envelope.OK || envelope.Data.Command == nil {
		t.Fatalf("wait body %s", body)
	}
	return *envelope.Data.Command
}

func postCommandResult(t *testing.T, serverURL, machineID string, result protocol.CommandResult) {
	t.Helper()
	raw, _ := json.Marshal(result)
	req, err := http.NewRequest(http.MethodPost, serverURL+protocol.ResultPath+"?agentCode=host&machineId="+machineID, strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", protocol.AuthorizationValue(setting.DevIngestPlain))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("result %d %s", res.StatusCode, body)
	}
}
