// Package shell 记录控制台打开的终端会话。
// 一行是一个终端。同一代理上可以同时有多个人、每人多个终端。
package shell

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"fluxguard/common/id"
	"fluxguard/common/logger"
	"fluxguard/server/agenttunnel"
	"fluxguard/server/utils/const/errcode"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

const (
	statusOpen   = "O"
	statusClosed = "C"
)

var (
	errNotHost  = errcode.New("shell.agent.not_host", "只接受主机代理")
	errNotFound = errcode.New("shell.not_found", "终端不存在")
	errClosed   = errcode.New("shell.closed", "终端已关闭")
	errDenied   = errcode.New("shell.denied", "只能关闭自己打开的终端")
	errUserFull = errcode.New("shell.user.full", "你在这台代理上打开的终端已达上限")
	errFull     = errcode.New("shell.agent.full", "这台代理上打开的终端已达上限")
)

// View 是一条打开中的终端。
type View struct {
	ShellID    string `json:"shellId"`
	AgentID    string `json:"agentId"`
	UserID     string `json:"userId"`
	UserName   string `json:"userName"`
	ShellTitle string `json:"shellTitle"`
	ShellNo    int    `json:"shellNo"`
	StatusFlag string `json:"statusFlag"`
}

// Store 读写 FG_SHELL。
type Store struct {
	db  *dbx.DB
	hub *agenttunnel.Hub
}

var (
	globalStore *Store
	globalOnce  sync.Once
)

// InitGlobal 初始化进程内终端库并接到会话隧道（只执行一次）。
func InitGlobal(conn *dbx.DB, hub *agenttunnel.Hub) *Store {
	globalOnce.Do(func() {
		globalStore = &Store{db: conn}
		if hub != nil {
			globalStore.BindTunnel(hub)
		}
	})
	return globalStore
}

// Global 返回 InitGlobal 后的终端库；未初始化时为 nil。
func Global() *Store { return globalStore }

// OpenStore 使用已经灌好脚本的库。测试可直接用；生产请 InitGlobal。
func OpenStore(conn *dbx.DB) *Store {
	if globalStore != nil {
		return globalStore
	}
	return &Store{db: conn}
}

// Open 为当前用户在这台代理上再开一个终端。已有终端保持打开。
func (s *Store) Open(ctx context.Context, tenantID, userID, userName, agentID string) (View, error) {
	agentID = strings.TrimSpace(agentID)
	var code, status string
	err := store.Scan(ctx, s.db, `
		SELECT COALESCE(agentCode, ''), statusFlag FROM FG_AGENT
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID}, &code, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return View{}, errNotFound
	}
	if err != nil {
		return View{}, err
	}
	if status != "Y" {
		return View{}, errNotHost
	}
	if code != "" && code != "host" {
		return View{}, errNotHost
	}
	userCount, err := s.count(ctx, tenantID, agentID, userID)
	if err != nil {
		return View{}, err
	}
	if userCount >= limits.ShellMaxPerUser {
		return View{}, errUserFull
	}
	agentCount, err := s.count(ctx, tenantID, agentID, "")
	if err != nil {
		return View{}, err
	}
	if agentCount >= limits.ShellMaxPerAgent {
		return View{}, errFull
	}
	now := store.FormatTime(time.Now().UTC())
	name := strings.TrimSpace(userName)
	if name == "" {
		name = userID
	}
	view := View{
		ShellID:    id.New(),
		AgentID:    agentID,
		UserID:     userID,
		UserName:   name,
		ShellNo:    userCount + 1,
		ShellTitle: fmt.Sprintf("终端 %d", userCount+1),
		StatusFlag: statusOpen,
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO FG_SHELL (
		  shellId, tenantId, agentId, userId, userName, shellTitle, shellNo, statusFlag,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @shellId, @tenantId, @agentId, @userId, @userName, @shellTitle, @shellNo, @statusFlag,
		  @now, @userId, @now, @userId, 'SHELL', 1, 'Y'
		)`, dbx.Args{
		"shellId": view.ShellID, "tenantId": tenantID, "agentId": agentID,
		"userId": userID, "userName": name, "shellTitle": view.ShellTitle,
		"shellNo": view.ShellNo, "statusFlag": statusOpen, "now": now,
	})
	if err != nil {
		return View{}, err
	}
	pipes.lane(view.ShellID)
	pipes.touch(view.ShellID)
	s.recordAudit(ctx, tenantID, view.ShellID, agentID, userID, name, eventOpen, view.ShellTitle)
	s.pushOpen(tenantID, agentID, view.ShellID)
	return view, nil
}

// Touch 记下这个人还在看自己的终端。页面还开着时会定期调用。
func (s *Store) Touch(ctx context.Context, tenantID, userID, agentID string) error {
	rows, err := s.db.Query(ctx, `
		SELECT shellId FROM FG_SHELL
		WHERE tenantId = @tenantId AND agentId = @agentId AND userId = @userId
		  AND statusFlag = @statusFlag AND activeFlag = 'Y'`,
		dbx.Args{
			"tenantId": tenantID, "agentId": strings.TrimSpace(agentID),
			"userId": userID, "statusFlag": statusOpen,
		})
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var shellID string
		if err := rows.Scan(&shellID); err != nil {
			return err
		}
		pipes.touch(shellID)
	}
	return rows.Err()
}

// List 返回这台代理上仍打开的终端，按打开时间排列。包含其他用户的终端。
func (s *Store) List(ctx context.Context, tenantID, agentID string) ([]View, error) {
	if err := s.dropIdle(ctx, tenantID, agentID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT shellId, agentId, userId, COALESCE(userName, ''), shellTitle, shellNo, statusFlag
		FROM FG_SHELL
		WHERE tenantId = @tenantId AND agentId = @agentId AND statusFlag = @statusFlag AND activeFlag = 'Y'
		ORDER BY addTime, shellId`,
		dbx.Args{"tenantId": tenantID, "agentId": strings.TrimSpace(agentID), "statusFlag": statusOpen})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []View
	for rows.Next() {
		var item View
		if err := rows.Scan(&item.ShellID, &item.AgentID, &item.UserID, &item.UserName, &item.ShellTitle, &item.ShellNo, &item.StatusFlag); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if out == nil {
		out = []View{}
	}
	return out, rows.Err()
}

// Close 关闭一个终端。打开人可以关自己的，租户管理员可以关任意一个。
func (s *Store) Close(ctx context.Context, tenantID, userID, shellID string, admin bool) error {
	var owner, status string
	err := store.Scan(ctx, s.db, `
		SELECT userId, statusFlag FROM FG_SHELL
		WHERE tenantId = @tenantId AND shellId = @shellId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "shellId": strings.TrimSpace(shellID)}, &owner, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	if status != statusOpen {
		return errClosed
	}
	if owner != userID && !admin {
		return errDenied
	}
	return s.remove(ctx, tenantID, strings.TrimSpace(shellID))
}

// Reap 定时清掉没人再看的终端。代理进程已经退出时，不会再来拉列表，所以不能只靠那一次清理。
func (s *Store) Reap(ctx context.Context) {
	ticker := time.NewTicker(limits.ShellReapEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.reapIdle(ctx); err != nil && ctx.Err() == nil {
				logger.Error(ctx, "终端清理: %v", err)
			}
		}
	}
}

func (s *Store) reapIdle(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT tenantId, agentId FROM FG_SHELL WHERE activeFlag = 'Y'`, nil)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pair struct{ tenantID, agentID string }
	var agents []pair
	for rows.Next() {
		var item pair
		if err := rows.Scan(&item.tenantID, &item.agentID); err != nil {
			return err
		}
		agents = append(agents, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range agents {
		if err := s.dropIdle(ctx, item.tenantID, item.agentID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) dropIdle(ctx context.Context, tenantID, agentID string) error {
	agentID = strings.TrimSpace(agentID)
	_, err := s.db.Exec(ctx, `
		DELETE FROM FG_SHELL
		WHERE tenantId = @tenantId AND agentId = @agentId AND statusFlag = @statusFlag`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID, "statusFlag": statusClosed})
	if err != nil {
		return err
	}
	rows, err := s.db.Query(ctx, `
		SELECT shellId FROM FG_SHELL
		WHERE tenantId = @tenantId AND agentId = @agentId AND statusFlag = @statusFlag AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID, "statusFlag": statusOpen})
	if err != nil {
		return err
	}
	defer rows.Close()
	var stale []string
	for rows.Next() {
		var shellID string
		if err := rows.Scan(&shellID); err != nil {
			return err
		}
		if pipes.idle(shellID, limits.ShellIdle) {
			stale = append(stale, shellID)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, shellID := range stale {
		if err := s.remove(ctx, tenantID, shellID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) remove(ctx context.Context, tenantID, shellID string) error {
	var agentID, userID, userName string
	err := store.Scan(ctx, s.db, `
		SELECT agentId, userId, COALESCE(userName, '') FROM FG_SHELL
		WHERE tenantId = @tenantId AND shellId = @shellId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "shellId": shellID}, &agentID, &userID, &userName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = s.db.Exec(ctx, `
		DELETE FROM FG_SHELL
		WHERE tenantId = @tenantId AND shellId = @shellId`,
		dbx.Args{"tenantId": tenantID, "shellId": shellID})
	if err != nil {
		return err
	}
	if agentID != "" && userID != "" {
		s.recordAudit(ctx, tenantID, shellID, agentID, userID, userName, eventClose, "")
	}
	s.pushClose(tenantID, agentID, shellID)
	pipes.shutdown(shellID)
	return nil
}

func (s *Store) count(ctx context.Context, tenantID, agentID, userID string) (int, error) {
	query := `
		SELECT COUNT(1) FROM FG_SHELL
		WHERE tenantId = @tenantId AND agentId = @agentId AND statusFlag = @statusFlag AND activeFlag = 'Y'`
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID, "statusFlag": statusOpen}
	if userID != "" {
		query += ` AND userId = @userId`
		args["userId"] = userID
	}
	var n int
	if err := store.Scan(ctx, s.db, query, args, &n); err != nil {
		return 0, err
	}
	return n, nil
}
