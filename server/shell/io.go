package shell

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/common/logger"
	"fluxguard/server/errcode"
	"fluxguard/server/limits"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

const maxInput = limits.ShellMaxInput

var (
	errNotOwner = errcode.New("shell.not_owner", "只能向自己打开的终端输入")
	errTooLong  = errcode.New("shell.input.long", "输入过长")
	errBuffer   = errcode.New("shell.input.full", "终端输入来不及处理")
	errBadSize  = errcode.New("shell.size", "终端尺寸无效")
)

// Chunk 是控制台从一路终端读到的新输出。
type Chunk struct {
	Text   string `json:"text"`
	Offset int    `json:"offset"`
	Closed bool   `json:"closed"`
}

// Input 是代理取走的一段键盘输入。
type Input struct {
	Data   string `json:"data"`
	Closed bool   `json:"closed"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

// WriteInput 把一行交给这路终端。只有打开人可以写。
func (s *Store) WriteInput(ctx context.Context, tenantID, userID, shellID, text string) error {
	shellID = strings.TrimSpace(shellID)
	owner, status, err := s.owner(ctx, tenantID, shellID)
	if err != nil {
		return err
	}
	if status != statusOpen {
		return errClosed
	}
	if owner != userID {
		return errNotOwner
	}
	pipes.touch(shellID)
	if len(text) > maxInput {
		return errTooLong
	}
	if text != "" && !pipes.lane(shellID).pushIn([]byte(text)) {
		return errBuffer
	}
	if line := pipes.lane(shellID).auditLine(text); line != "" {
		agentID, userName, metaErr := s.shellMeta(ctx, tenantID, shellID)
		if metaErr != nil {
			logger.Warn(ctx, "终端审计查会话失败 shell=%s err=%v", shellID, metaErr)
		} else {
			s.recordAudit(ctx, tenantID, shellID, agentID, userID, userName, eventInput, line)
		}
		logger.Info(ctx, "终端输入 shell=%s user=%s %s", shellID, userID, clipRunes(line, 120))
	}
	return nil
}

// WriteSize 记下这一路终端的行列，代理取输入时一并拿走并调整屏幕。
func (s *Store) WriteSize(ctx context.Context, tenantID, userID, shellID string, cols, rows int) error {
	shellID = strings.TrimSpace(shellID)
	owner, status, err := s.owner(ctx, tenantID, shellID)
	if err != nil {
		return err
	}
	if status != statusOpen {
		return errClosed
	}
	if owner != userID {
		return errNotOwner
	}
	pipes.touch(shellID)
	if !pipes.lane(shellID).setSize(cols, rows) {
		return errBadSize
	}
	return nil
}

// ReadOutput 返回打开人尚未读过的输出。
func (s *Store) ReadOutput(ctx context.Context, tenantID, userID, shellID string, offset int) (Chunk, error) {
	shellID = strings.TrimSpace(shellID)
	owner, status, err := s.owner(ctx, tenantID, shellID)
	if err != nil {
		return Chunk{}, err
	}
	if owner != userID {
		return Chunk{}, errNotOwner
	}
	pipes.touch(shellID)
	lane := pipes.get(shellID)
	if lane == nil {
		return Chunk{Offset: offset, Closed: status != statusOpen}, nil
	}
	text, next, closed, skipped := lane.readOut(offset)
	if status != statusOpen {
		closed = true
	}
	if skipped {
		text = "[较早的输出已丢弃]\n" + text
	}
	return Chunk{Text: text, Offset: next, Closed: closed}, nil
}

// TakeInput 给代理取走输入。shell 必须属于这台代理。
func (s *Store) TakeInput(ctx context.Context, tenantID, agentID, shellID string, wait time.Duration) (Input, error) {
	shellID = strings.TrimSpace(shellID)
	open, err := s.ownedByAgent(ctx, tenantID, agentID, shellID)
	if err != nil {
		return Input{}, err
	}
	if !open {
		pipes.shutdown(shellID)
		return Input{Closed: true}, nil
	}
	data, closed, cols, rows := pipes.lane(shellID).takeIn(ctx, wait)
	if !closed {
		open, err = s.ownedByAgent(ctx, tenantID, agentID, shellID)
		if err != nil {
			return Input{}, err
		}
		closed = !open
	}
	return Input{Data: string(data), Closed: closed, Cols: cols, Rows: rows}, nil
}

// PutOutput 写入代理送回的输出。
func (s *Store) PutOutput(ctx context.Context, tenantID, agentID, shellID, data string) error {
	shellID = strings.TrimSpace(shellID)
	open, err := s.ownedByAgent(ctx, tenantID, agentID, shellID)
	if err != nil {
		return err
	}
	if !open {
		return errClosed
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	pipes.lane(shellID).pushOut([]byte(data))
	return nil
}

func (s *Store) owner(ctx context.Context, tenantID, shellID string) (string, string, error) {
	var owner, status string
	err := store.Scan(ctx, s.db, `
		SELECT userId, statusFlag FROM FG_SHELL
		WHERE tenantId = @tenantId AND shellId = @shellId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "shellId": strings.TrimSpace(shellID)}, &owner, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", errNotFound
	}
	return owner, status, err
}

func (s *Store) shellMeta(ctx context.Context, tenantID, shellID string) (agentID, userName string, err error) {
	err = store.Scan(ctx, s.db, `
		SELECT agentId, COALESCE(userName, '') FROM FG_SHELL
		WHERE tenantId = @tenantId AND shellId = @shellId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "shellId": strings.TrimSpace(shellID)}, &agentID, &userName)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", errNotFound
	}
	return agentID, userName, err
}

func (s *Store) ownedByAgent(ctx context.Context, tenantID, agentID, shellID string) (bool, error) {
	var ownerAgent, status string
	err := store.Scan(ctx, s.db, `
		SELECT agentId, statusFlag FROM FG_SHELL
		WHERE tenantId = @tenantId AND shellId = @shellId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "shellId": strings.TrimSpace(shellID)}, &ownerAgent, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return ownerAgent == agentID && status == statusOpen, nil
}
