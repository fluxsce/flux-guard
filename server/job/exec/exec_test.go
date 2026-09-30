package exec

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"fluxguard/server/job/contract"
	"fluxguard/server/job/policy"
	"fluxguard/server/job/registry"
	"fluxguard/server/job/status"
)

type probeJob struct {
	got contract.Call
}

func (p *probeJob) Meta() contract.Meta {
	return contract.Meta{Code: "probe-job", Title: "探针", Kind: "probe"}
}

func (p *probeJob) Run(_ context.Context, call contract.Call) (contract.Result, error) {
	p.got = call
	return contract.Result{Data: []byte(`{"n":1}`)}, nil
}

func TestRunPassesParams(t *testing.T) {
	job := &probeJob{}
	registry.Register(job)
	policy.Put("default", "probe-job", json.RawMessage(`{"enabled":true,"mode":"manual","params":{"n":1}}`))
	res, err := Run(context.Background(), "probe-job", "default", contract.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Data) != `{"n":1}` || string(job.got.Params) != `{"n":1}` || job.got.Trigger != contract.TriggerManual {
		t.Fatalf("res=%#v call=%#v", res, job.got)
	}
	st, ok := status.Get("default", "probe-job")
	if !ok || !st.OK || !st.RanAtLeast || string(st.Result) != `{"n":1}` || st.Trigger != "manual" || st.DurationMs < 0 {
		t.Fatalf("%#v", st)
	}
	policy.Put("default", "probe-job", json.RawMessage(`{"enabled":false,"mode":"manual","params":{}}`))
	if _, err := Run(context.Background(), "probe-job", "default", contract.TriggerManual); err == nil {
		t.Fatal("disabled job should not run")
	}
}

type holdJob struct {
	started chan struct{}
	release chan struct{}
}

func (h *holdJob) Meta() contract.Meta {
	return contract.Meta{Code: "hold-job", Title: "占用"}
}

func (h *holdJob) Run(ctx context.Context, _ contract.Call) (contract.Result, error) {
	close(h.started)
	select {
	case <-ctx.Done():
		return contract.Result{}, ctx.Err()
	case <-h.release:
		return contract.Result{Data: []byte(`{}`)}, nil
	}
}

func TestBusyRejectsOverlap(t *testing.T) {
	job := &holdJob{started: make(chan struct{}), release: make(chan struct{})}
	registry.Register(job)
	policy.Put("default", "hold-job", json.RawMessage(`{"enabled":true,"mode":"manual","timeoutSeconds":30,"retryCount":0,"params":{}}`))
	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), "hold-job", "default", contract.TriggerManual)
		done <- err
	}()
	select {
	case <-job.started:
	case <-time.After(2 * time.Second):
		t.Fatal("first run did not start")
	}
	_, err := Run(context.Background(), "hold-job", "default", contract.TriggerManual)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("overlap: %v", err)
	}
	close(job.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type flakyJob struct {
	n int
}

func (f *flakyJob) Meta() contract.Meta {
	return contract.Meta{Code: "flaky-job", Title: "重试"}
}

func (f *flakyJob) Run(context.Context, contract.Call) (contract.Result, error) {
	f.n++
	if f.n == 1 {
		return contract.Result{}, errors.New("fail")
	}
	return contract.Result{Data: []byte(`{"ok":true}`)}, nil
}

func TestRetryThenSuccess(t *testing.T) {
	job := &flakyJob{}
	registry.Register(job)
	policy.Put("default", "flaky-job", json.RawMessage(`{"enabled":true,"mode":"manual","timeoutSeconds":30,"retryCount":1,"params":{}}`))
	res, err := Run(context.Background(), "flaky-job", "default", contract.TriggerManual)
	if err != nil || job.n != 2 || string(res.Data) != `{"ok":true}` {
		t.Fatalf("n=%d err=%v res=%s", job.n, err, res.Data)
	}
	st, ok := status.Get("default", "flaky-job")
	if !ok || !st.OK {
		t.Fatalf("%#v", st)
	}
}

type blockJob struct{}

func (blockJob) Meta() contract.Meta {
	return contract.Meta{Code: "block-job", Title: "超时"}
}

func (blockJob) Run(ctx context.Context, _ contract.Call) (contract.Result, error) {
	<-ctx.Done()
	return contract.Result{}, ctx.Err()
}

func TestCallTimeout(t *testing.T) {
	registry.Register(blockJob{})
	policy.Put("default", "block-job", json.RawMessage(`{"enabled":true,"mode":"manual","timeoutSeconds":1,"retryCount":0,"params":{}}`))
	_, err := Run(context.Background(), "block-job", "default", contract.TriggerManual)
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatal(err)
	}
	st, ok := status.Get("default", "block-job")
	if !ok || st.OK {
		t.Fatalf("%#v", st)
	}
}
