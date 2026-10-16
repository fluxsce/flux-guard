package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRestartCommandStopsThenStarts(t *testing.T) {
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		if cmdline == "./status.sh" {
			return errors.New("down")
		}
		return nil
	}
	t.Cleanup(func() { shellRunner = prev })

	rt := Runtime{Kind: "command", Start: "./startup.sh", Stop: "./shutdown.sh", Status: "./status.sh"}
	if err := Restart(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 || calls[0] != "./shutdown.sh" || calls[len(calls)-1] != "./startup.sh" {
		t.Fatalf("%v", calls)
	}
}

func TestRestartWithoutStopKillsThenStarts(t *testing.T) {
	alive := true
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		switch cmdline {
		case "./status.sh":
			if alive {
				return nil
			}
			return errors.New("down")
		case "./kill.sh":
			alive = false
			return nil
		default:
			return nil
		}
	}
	t.Cleanup(func() { shellRunner = prev })

	rt := Runtime{
		Kind: "command", Start: "./startup.sh", Status: "./status.sh", Kill: "./kill.sh",
	}
	if err := Restart(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 || calls[0] == "./shutdown.sh" || calls[len(calls)-1] != "./startup.sh" {
		t.Fatalf("%v", calls)
	}
	sawKill := false
	for _, call := range calls {
		if call == "./kill.sh" {
			sawKill = true
		}
	}
	if !sawKill {
		t.Fatalf("%v", calls)
	}
}

func TestRestartCommandUsesRestartScript(t *testing.T) {
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		return nil
	}
	t.Cleanup(func() { shellRunner = prev })

	rt := Runtime{
		Kind: "command", Start: "./startup.sh", Stop: "./shutdown.sh", Restart: "./restart.sh",
	}
	if err := Restart(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != "./restart.sh" {
		t.Fatalf("%v", calls)
	}
}

func TestRestartCommandContinuesAfterStopTimeout(t *testing.T) {
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		if cmdline == "./shutdown.sh" {
			<-ctx.Done()
			return ctx.Err()
		}
		if cmdline == "./status.sh" {
			return errors.New("down")
		}
		return nil
	}
	t.Cleanup(func() { shellRunner = prev })

	old := StopGraceTimeout
	StopGraceTimeout = 40 * time.Millisecond
	t.Cleanup(func() { StopGraceTimeout = old })

	rt := Runtime{Kind: "command", Start: "./startup.sh", Stop: "./shutdown.sh", Status: "./status.sh"}
	if err := Restart(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if calls[0] != "./shutdown.sh" || calls[len(calls)-1] != "./startup.sh" {
		t.Fatalf("%v", calls)
	}
}

func TestStopCommandForceKillsWhenStillRunning(t *testing.T) {
	alive := true
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		switch cmdline {
		case "./shutdown.sh":
			return nil
		case "./status.sh":
			if alive {
				return nil
			}
			return errors.New("down")
		case "./kill.sh":
			alive = false
			return nil
		default:
			return nil
		}
	}
	t.Cleanup(func() { shellRunner = prev })

	old := StopGraceTimeout
	StopGraceTimeout = 50 * time.Millisecond
	t.Cleanup(func() { StopGraceTimeout = old })

	rt := Runtime{
		Kind: "command", Stop: "./shutdown.sh", Status: "./status.sh", Kill: "./kill.sh",
	}
	if err := Stop(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	hasKill := false
	for _, c := range calls {
		if c == "./kill.sh" {
			hasKill = true
		}
	}
	if !hasKill {
		t.Fatalf("expected kill, got %v", calls)
	}
}

func TestStartStopCommand(t *testing.T) {
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		if cmdline == "./status.sh" {
			return errors.New("down")
		}
		return nil
	}
	t.Cleanup(func() { shellRunner = prev })

	rt := Runtime{Kind: "command", Mode: "tomcat", Start: "./startup.sh", Stop: "./shutdown.sh", Status: "./status.sh"}
	if err := Start(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if err := Stop(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if calls[0] != "./startup.sh" || calls[1] != "./shutdown.sh" {
		t.Fatalf("%v", calls)
	}
	if err := Start(context.Background(), Runtime{Kind: "command", Mode: "files", Start: "./x"}); err == nil {
		t.Fatal("files mode should reject start")
	}
}

func TestRunningCommandUsesStatus(t *testing.T) {
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		if cmdline == "./status.sh" {
			return nil
		}
		return errors.New("unexpected")
	}
	t.Cleanup(func() { shellRunner = prev })

	rt := Runtime{Kind: "command", Status: "./status.sh"}
	if !Running(context.Background(), rt) {
		t.Fatal("expected running")
	}
	shellRunner = func(ctx context.Context, cmdline string) error {
		return errors.New("down")
	}
	if Running(context.Background(), rt) {
		t.Fatal("expected down")
	}
}
