// Package inventory 编排 files 模式目录清单刷新：能拨则短拨；拨不通则出站推清单（隔离网）。
package inventory

import (
	"context"
	"errors"
	"strings"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/command"
	"fluxguard/server/helper/applist"
	"fluxguard/server/web/views/fg0008/dao"
	dbx "github.com/fluxsce/dbx/db"
)

// Refresh 刷新一套 files 运行时的文件清单并写库。
// 拨通：中央短拨列目录后同步。拨不通：下发 app.file.list，等代理出站推清单后再同步。
func Refresh(ctx context.Context, db *dbx.DB, tenantID, user, agentID, instance string) (mode string, err error) {
	agentID = strings.TrimSpace(agentID)
	instance = strings.TrimSpace(instance)
	if agentID == "" || instance == "" {
		return "", errors.New("缺少运行时")
	}
	mode, err = dao.SiteMode(ctx, db, tenantID, agentID, instance)
	if err != nil {
		return "", err
	}
	if mode != "files" {
		return mode, nil
	}
	svc := command.Current()
	if svc == nil {
		return mode, errors.New("命令通道未启用")
	}
	if items, dialErr := svc.PullAppsideFileList(ctx, tenantID, agentID, instance); dialErr == nil {
		return mode, dao.SyncFilesInventory(ctx, db, tenantID, agentID, instance, items)
	}

	view, err := svc.Dispatch(ctx, tenantID, user, command.DispatchInput{
		AgentID: agentID, Action: protocol.ActionAppFileList, Instance: instance,
	})
	if err != nil {
		return mode, err
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		st, getErr := svc.Get(ctx, tenantID, view.CommandID)
		if getErr != nil {
			return mode, getErr
		}
		switch st.Status {
		case "done":
			items, loadErr := applist.Load(tenantID, view.CommandID)
			if loadErr != nil {
				return mode, errors.New("代理已回执但清单未到达")
			}
			syncErr := dao.SyncFilesInventory(ctx, db, tenantID, agentID, instance, items)
			applist.Remove(tenantID, view.CommandID)
			return mode, syncErr
		case "failed":
			applist.Remove(tenantID, view.CommandID)
			msg := strings.TrimSpace(st.Message)
			if msg == "" {
				msg = "代理列目录失败"
			}
			return mode, errors.New(msg)
		}
		// 清单可能先于回执到达：就绪则提前同步，仍等命令结束以免重复下发。
		if applist.Ready(tenantID, view.CommandID) {
			if items, loadErr := applist.Load(tenantID, view.CommandID); loadErr == nil {
				_ = dao.SyncFilesInventory(ctx, db, tenantID, agentID, instance, items)
			}
		}
		select {
		case <-ctx.Done():
			return mode, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return mode, errors.New("等待代理推送目录清单超时（隔离网须代理能访问中央）")
}
