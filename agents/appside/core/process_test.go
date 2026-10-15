package core

import (
	"context"
	"errors"
	"testing"
)

func TestRestartCommandStopsThenStarts(t *testing.T) {
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		return nil
	}
	t.Cleanup(func() { shellRunner = prev })

	rt := Runtime{Kind: "command", Start: "./startup.sh", Stop: "./shutdown.sh", Status: "./status.sh"}
	if err := Restart(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "./shutdown.sh" || calls[1] != "./startup.sh" {
		t.Fatalf("%v", calls)
	}
}

func TestStartStopCommand(t *testing.T) {
	var calls []string
	prev := shellRunner
	shellRunner = func(ctx context.Context, cmdline string) error {
		calls = append(calls, cmdline)
		return nil
	}
	t.Cleanup(func() { shellRunner = prev })

	rt := Runtime{Kind: "command", Mode: "tomcat", Start: "./startup.sh", Stop: "./shutdown.sh"}
	if err := Start(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if err := Stop(context.Background(), rt); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "./startup.sh" || calls[1] != "./shutdown.sh" {
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
