// Package exec 按编码调用已登记的 Job。
// 同一租户的同一任务同时只允许一次调用。未启用的任务直接返回错误，不调用、不写日志。
// 单次调用按策略超时；失败后按策略重试。结束后写入 status：OK 来自最后一次 error，耗时覆盖全部尝试。
package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
	"fluxguard/server/job/policy"
	"fluxguard/server/job/registry"
	"fluxguard/server/job/status"
	"fluxguard/server/utils/const/limits"
)

// ErrBusy 同一任务已有一次调用尚未结束。
var ErrBusy = errors.New("任务正在执行")

var inflight sync.Map

// Run 按当前策略取出参数对象，调用对应 Job，并回写最近执行与日志。
func Run(ctx context.Context, code, tenantID string, trigger contract.Trigger) (contract.Result, error) {
	spec, ok := catalog.ByCode(code)
	if !ok {
		return contract.Result{}, fmt.Errorf("未知任务 %s", code)
	}
	job, ok := registry.Get(code)
	if !ok {
		return contract.Result{}, fmt.Errorf("任务 %s 未注册", code)
	}
	if trigger == "" {
		trigger = contract.TriggerManual
	}
	if spec.Scope == catalog.ScopeSystem {
		tenantID = catalog.SystemTenant
	} else {
		tenantID = policy.TenantKey(tenantID)
	}
	raw := policy.Get(tenantID, spec.Code)
	sched := policy.DecodeSchedule(raw)
	if !sched.Enabled {
		return contract.Result{}, fmt.Errorf("任务未启用")
	}
	key := policy.Key(tenantID, spec.Code)
	if _, loaded := inflight.LoadOrStore(key, struct{}{}); loaded {
		return contract.Result{}, ErrBusy
	}
	defer inflight.Delete(key)

	call := contract.Call{
		TenantID: tenantID,
		Trigger:  trigger,
		Params:   policy.Params(raw),
	}
	timeout := time.Duration(sched.TimeoutSeconds) * time.Second
	attempts := sched.RetryCount + 1
	if attempts < 1 {
		attempts = 1
	}
	start := time.Now().UTC()
	res, err := attempt(ctx, job, call, timeout, attempts)
	end := time.Now().UTC()
	if len(res.Data) == 0 {
		res.Data = json.RawMessage(`{}`)
	}
	st := status.Status{
		TenantID:   tenantID,
		Code:       spec.Code,
		Title:      spec.Title,
		Enabled:    sched.Enabled,
		Trigger:    string(trigger),
		LastStart:  start,
		LastEnd:    end,
		DurationMs: end.Sub(start).Milliseconds(),
		OK:         err == nil,
		Result:     append(json.RawMessage(nil), res.Data...),
		RanAtLeast: true,
	}
	if err != nil {
		st.ErrorText = err.Error()
		st.OK = false
	}
	status.Report(st)
	return res, err
}

// attempt 在超时内调用，失败则等待后重试。父上下文取消时不再重试。
func attempt(ctx context.Context, job contract.Job, call contract.Call, timeout time.Duration, attempts int) (contract.Result, error) {
	var last contract.Result
	var err error
	for i := 0; i < attempts; i++ {
		if err = ctx.Err(); err != nil {
			return last, err
		}
		last, err = callOnce(ctx, job, call, timeout)
		if err == nil {
			return last, nil
		}
		if ctx.Err() != nil || i == attempts-1 {
			return last, err
		}
		timer := time.NewTimer(limits.JobRetryPause)
		select {
		case <-ctx.Done():
			timer.Stop()
			return last, ctx.Err()
		case <-timer.C:
		}
	}
	return last, err
}

// callOnce 给这一次尝试单独的超时，结束时取消，避免上下文泄漏。
func callOnce(ctx context.Context, job contract.Job, call contract.Call, timeout time.Duration) (contract.Result, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := job.Run(runCtx, call)
	if ctx.Err() != nil {
		return res, err
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		if err == nil {
			return res, fmt.Errorf("任务执行超时（%s）", timeout)
		}
		return res, fmt.Errorf("任务执行超时（%s）: %w", timeout, err)
	}
	return res, err
}
