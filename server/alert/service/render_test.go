package service

import (
	"context"
	"strings"
	"testing"

	"fluxguard/server/alert/channel"
	"fluxguard/server/alert/models"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

func systemCtx() context.Context {
	return request.With(context.Background(), request.Principal{TenantID: "default", UserID: "system"})
}

func TestDingSignStable(t *testing.T) {
	ts, sign := DingSign("SEC", 1599360473000)
	if ts != "1599360473000" || sign == "" {
		t.Fatalf("加签 %s %s", ts, sign)
	}
	_, again := DingSign("SEC", 1599360473000)
	if again != sign {
		t.Fatal("同一时间的加签应稳定")
	}
}

func TestChannelSecretAndBuiltinTemplate(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := Open(db)
	channelRow, err := svc.SaveChannel(systemCtx(), models.ChannelInput{
		ChannelName: "邮件", ChannelKind: "email", Endpoint: "smtp.example.com", Port: 465,
		Username: "ops@example.com", Secret: "secret-pass", Sender: "ops@example.com",
		Target: "oncall@example.com", StatusFlag: "Y",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if channelRow.SecretSet != "Y" || channelRow.Endpoint != "smtp.example.com" {
		t.Fatalf("渠道视图 %#v", channelRow)
	}
	var stored string
	if err := store.Scan(context.Background(), db, `
		SELECT configText FROM FG_ALERT_CHANNEL WHERE tenantId = 'default' AND channelId = @channelId`,
		dbx.Args{"channelId": channelRow.ChannelID}, &stored); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, "ENCY_") || strings.Contains(stored, "secret-pass") {
		t.Fatalf("密钥应加密保存: %s", stored)
	}
	cfg := channel.ParseConfig(stored)
	if channel.PlainSecret(cfg, "password") != "secret-pass" {
		t.Fatalf("还原密钥失败 %q", channel.PlainSecret(cfg, "password"))
	}
	if channelRow.TemplateID != "tpl-email" || !strings.Contains(stored, `"templateId":"tpl-email"`) {
		t.Fatalf("应关联邮件内置模板 %#v %s", channelRow, stored)
	}
}
