// Package service 实现预警模板、通知渠道和事件发送。
// 管理页是 fg0041 模板、fg0042 渠道、fg0043 事件。功能模块写入事件后，本包轮询事件表按渠道模板发送。
package service

import (
	"context"
	"sync"
	"time"

	"fluxguard/server/alert/channel"
	"fluxguard/server/alert/event"
	"fluxguard/server/alert/rule"
	"fluxguard/server/alert/template"
	"fluxguard/server/limits"
	dbx "github.com/fluxsce/dbx/db"
)

// Service 接收预警事件，并按渠道模板发送。不读取各功能的指标表。
type Service struct {
	db       *dbx.DB
	eval     sync.Mutex
	tpl      *template.Store
	channels *channel.Store
	events   *event.Store
	rules    *rule.Store
}

// DB 返回预警服务使用的库连接。
func (s *Service) DB() *dbx.DB {
	if s == nil {
		return nil
	}
	return s.db
}

var current *Service

// Current 返回本进程预警服务。尚未 Open 时为 nil。
func Current() *Service {
	return current
}

// Open 打开预警服务。
// 同一条库连接再次打开时返回已有实例，扫描循环和管理页共用它。
func Open(db *dbx.DB) *Service {
	if db == nil {
		return nil
	}
	if current != nil && current.db == db {
		return current
	}
	tpl := template.NewStore(db)
	current = &Service{
		db:       db,
		tpl:      tpl,
		channels: channel.NewStore(db, tpl),
		events:   event.NewStore(db),
		rules:    rule.NewStore(db),
	}
	return current
}

// Loop 立即按预警日志投递一轮，之后每 30 秒一轮，直到 ctx 结束。
// 每一轮都带着 ctx。取消后当前查询会结束，计时器在返回前停止。
func (s *Service) Loop(ctx context.Context) {
	if s == nil {
		return
	}
	s.Evaluate(ctx)
	ticker := time.NewTicker(limits.AlertScanEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Evaluate(ctx)
		}
	}
}
