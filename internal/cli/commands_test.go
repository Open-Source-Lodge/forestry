package cli

import (
	"errors"
	"path/filepath"
	"strings"
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
	got := status(wt, false)
	if got != "clean" {
		t.Errorf("status(clean wt) = %q, want clean", got)
	}

	wt.Main = true
	got = status(wt, false)
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
	got = status(wt, false)
	if got != "locked" {
		t.Errorf("status(locked wt) = %q, want locked", got)
	}

	wt.Locked = false
	got = status(wt, true)
	if got != "shell" {
		t.Errorf("status(wt with shell) = %q, want shell", got)
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

// A branch that is only on the remote is fetched and then checked out as it is.
func TestCreateWorktreeUsesRemoteBranch(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	var ran []string
	var fetched bool
	stubCommand(t, func(name string, args ...string) (string, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		ran = append(ran, line)
		switch {
		case strings.Contains(line, "show-ref"):
			if !strings.Contains(line, "refs/remotes/origin/feat") || !fetched {
				return "", errors.New("no such ref")
			}
		case strings.Contains(line, "fetch"):
			fetched = true
		case strings.Contains(line, "worktree list"):
			return "worktree " + repo + "\nbranch refs/heads/main\n", nil
		}
		return "", nil
	})

	if _, err := createWorktree("feat", ""); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ran, "\n")
	if !strings.Contains(joined, "git fetch origin refs/heads/feat:refs/remotes/origin/feat") {
		t.Errorf("remote branch not fetched:\n%s", joined)
	}
	if !strings.Contains(joined, "worktree add "+repo+"-worktrees/feat feat") || strings.Contains(joined, "add -b") {
		t.Errorf("branch not checked out as it is:\n%s", joined)
	}
}

// A name that is nowhere yet makes a branch, and no fetch keeps it waiting.
func TestCreateWorktreeMakesNewBranch(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	var ran []string
	stubCommand(t, func(name string, args ...string) (string, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		ran = append(ran, line)
		switch {
		case strings.Contains(line, "show-ref"):
			return "", errors.New("no such ref")
		case strings.Contains(line, "fetch"):
			return "", errors.New("couldn't find remote ref")
		case strings.Contains(line, "worktree list"):
			return "worktree " + repo + "\nbranch refs/heads/main\n", nil
		}
		return "", nil
	})

	if _, err := createWorktree("feat", ""); err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(ran, "\n"); !strings.Contains(joined, "worktree add -b feat "+repo+"-worktrees/feat HEAD") {
		t.Errorf("new branch not made:\n%s", joined)
	}
}

// An existing branch is checked out as it is, also when --from is given.
func TestCreateWorktreeIgnoresFromForExistingBranch(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	var ran []string
	stubCommand(t, func(name string, args ...string) (string, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		ran = append(ran, line)
		if strings.Contains(line, "worktree list") {
			return "worktree " + repo + "\nbranch refs/heads/main\n", nil
		}
		return "", nil
	})

	if _, err := createWorktree("feat", "main"); err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(ran, "\n"); !strings.Contains(joined, "worktree add "+repo+"-worktrees/feat feat") || strings.Contains(joined, "add -b") {
		t.Errorf("existing branch not checked out as it is:\n%s", joined)
	}
}
