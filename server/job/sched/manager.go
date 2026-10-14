// Package sched 在单个进程里按固定节拍轮询已登记任务。
// 手动方式不自动执行。按间隔到期、指定时间到达，或当前分钟命中 cron，才调用 exec.Run。
// 每个租户单独调用一次。任务仍在执行时本拍跳过，不把这次算作已跑。失败写完日志后才记下时间。
package sched

import (
	"context"
	"errors"
	"sync"
	"time"

	"fluxguard/common/logger"
	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
	"fluxguard/server/job/exec"
	"fluxguard/server/job/policy"
	"fluxguard/server/job/registry"
	"fluxguard/server/job/status"
	"fluxguard/server/utils/const/limits"
)

// TenantLister 列举参与租户级任务的租户。
type TenantLister func(ctx context.Context) ([]string, error)

// Manager 统一调度。
type Manager struct {
	tenants  TenantLister
	fallback string

	mu      sync.Mutex
	lastRun map[string]time.Time
}

// New 创建调度器。fallback 在租户列表为空时使用。
func New(tenants TenantLister, fallbackTenant string) *Manager {
	if tenants == nil {
		tenants = func(context.Context) ([]string, error) { return nil, nil }
	}
	return &Manager{
		tenants:  tenants,
		fallback: policy.TenantKey(fallbackTenant),
		lastRun:  map[string]time.Time{},
	}
}

// Start 阻塞运行，直到 ctx 取消。
func (m *Manager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	m.tick(ctx)
	ticker := time.NewTicker(limits.JobTickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

func (m *Manager) tick(ctx context.Context) {
	tenants, err := m.listTenants(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.Error(ctx, "任务调度读租户: %v", err)
		}
		return
	}
	now := time.Now().UTC()
	for _, spec := range catalog.Specs() {
		if _, ok := registry.Get(spec.Code); !ok {
			continue
		}
		switch spec.Scope {
		case catalog.ScopeSystem:
			m.fire(ctx, spec, catalog.SystemTenant, now)
		default:
			for _, tid := range tenants {
				m.fire(ctx, spec, tid, now)
			}
		}
	}
}

func (m *Manager) fire(ctx context.Context, spec catalog.Spec, tenantID string, now time.Time) {
	sched := policy.DecodeSchedule(policy.Get(tenantID, spec.Code))
	status.Touch(tenantID, spec.Code, spec.Title, sched.Enabled)
	if !sched.Enabled || !m.due(tenantID, spec.Code, sched, now) {
		return
	}
	trigger := contract.Trigger(sched.Mode)
	if trigger != contract.TriggerInterval && trigger != contract.TriggerAt && trigger != contract.TriggerCron {
		return
	}
	_, err := exec.Run(ctx, spec.Code, tenantID, trigger)
	if errors.Is(err, exec.ErrBusy) {
		return
	}
	if err != nil && ctx.Err() == nil {
		logger.Error(ctx, "任务 %s 执行失败: %v", spec.Code, err)
	}
	m.mark(tenantID, spec.Code, now)
}

func (m *Manager) due(tenantID, code string, sched policy.Schedule, now time.Time) bool {
	next, ok := policy.NextStart(sched, m.last(tenantID, code), now)
	return ok && !now.Before(next)
}

func (m *Manager) last(tenantID, code string) time.Time {
	key := policy.Key(tenantID, code)
	m.mu.Lock()
	last := m.lastRun[key]
	m.mu.Unlock()
	if last.IsZero() {
		if st, ok := status.Get(tenantID, code); ok {
			last = st.LastStart
			if last.IsZero() {
				last = st.LastEnd
			}
		}
	}
	return last
}

func (m *Manager) mark(tenantID, code string, at time.Time) {
	m.mu.Lock()
	m.lastRun[policy.Key(tenantID, code)] = at
	m.mu.Unlock()
}

func (m *Manager) listTenants(ctx context.Context) ([]string, error) {
	ids, err := m.tenants(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []string{m.fallback}, nil
	}
	return ids, nil
}
