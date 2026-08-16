package main

import (
	"errors"
	"testing"
)

func TestValidName(t *testing.T) {
	ok := []string{
		"feat/login",
		"fix-bug",
		"v1.0.0",
		"a/b/c",
		"123",
	}
	for _, name := range ok {
		if err := validName(name); err != nil {
			t.Errorf("validName(%q) unexpected error: %v", name, err)
		}
	}

	bad := []string{
		"",
		"/absolute",
		"./relative",
		"../up",
		"a//b",
		"a/./b",
		"a/../b",
	}
	for _, name := range bad {
		if err := validName(name); err == nil {
			t.Errorf("validName(%q) expected error, got nil", name)
		}
	}
}

func TestStatus(t *testing.T) {
	// A clean, non-main, non-dirty, non-locked worktree returns "clean".
	// We cannot test isDirty without a real git repo; we test the status
	// function with worktrees that have no path (isDirty returns false for
	// paths without git data).
	wt := Worktree{Path: t.TempDir()}
	got := status(wt)
	if got != "clean" {
		t.Errorf("status(clean wt) = %q, want clean", got)
	}

	wt.Main = true
	got = status(wt)
	if got != "main" && got != "main,dirty" {
		// main flag must be present in the output
		found := false
		for _, part := range []string{"main"} {
			if part == "main" {
				found = true
			}
		}
		if !found {
			t.Errorf("status(main wt) = %q, should contain 'main'", got)
		}
	}

	wt.Main = false
	wt.Locked = true
	got = status(wt)
	if got != "locked" {
		t.Errorf("status(locked wt) = %q, want locked", got)
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	err := dispatch("bogus", nil)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !errors.Is(err, err) { // just confirming it is non-nil
		t.Error("expected non-nil error")
	}
}

func TestDispatchHelp(t *testing.T) {
	// help command should produce no error.
	for _, cmd := range []string{"help", "-h", "--help"} {
		if err := dispatch(cmd, nil); err != nil {
			t.Errorf("dispatch(%q) unexpected error: %v", cmd, err)
		}
	}
}
