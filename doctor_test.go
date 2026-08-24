package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExistingAncestor(t *testing.T) {
	dir := t.TempDir()
	if got := existingAncestor(dir); got != dir {
		t.Errorf("existing dir: got %q, want %q", got, dir)
	}
	deep := filepath.Join(dir, "a", "b", "c")
	if got := existingAncestor(deep); got != dir {
		t.Errorf("missing dir: got %q, want %q", got, dir)
	}
	if got := existingAncestor("/nope/nowhere"); got != "/" {
		t.Errorf("absent path: got %q, want %q", got, "/")
	}
}

// A check can only depend on one listed above it, or the skip never triggers.
func TestChecksDependOnEarlierChecks(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range checks {
		if c.needs != "" && !seen[c.needs] {
			t.Errorf("check %q needs %q, which is not defined before it", c.name, c.needs)
		}
		seen[c.name] = true
	}
}

// A file we cannot look at is a different problem from one that is not there,
// and doctor exists to tell them apart.
func TestCheckShellSeparatesMissingFromUnreadable(t *testing.T) {
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	shell := filepath.Join(locked, "sh")
	if _, err := os.Stat(shell); errors.Is(err, os.ErrNotExist) {
		t.Skip("stat sees through a 000 directory — running as root?")
	}
	t.Setenv("SHELL", shell)
	_, err := checkShell()
	if err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("unreadable shell: got %v, want an unreadable error", err)
	}

	t.Setenv("SHELL", filepath.Join(t.TempDir(), "nope"))
	if _, err := checkShell(); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("missing shell: got %v, want a does-not-exist error", err)
	}
}

// Every check must say something, whether it passes or fails.
func TestChecksSayWhy(t *testing.T) {
	for _, c := range checks {
		detail, err := c.run()
		if err == nil && detail == "" {
			t.Errorf("check %q passed with no detail", c.name)
		}
		if err != nil && err.Error() == "" {
			t.Errorf("check %q failed with no reason", c.name)
		}
	}
}

func TestCheckConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubCommand(t, func(name string, args ...string) (string, error) {
		return t.TempDir(), nil // a repo with no .forestry
	})
	if detail, err := checkConfig(); err != nil || !strings.Contains(detail, "no .forestry") {
		t.Errorf("no file: %q, %v", detail, err)
	}
	path := filepath.Join(home, ".forestry")
	os.WriteFile(path, []byte("# nothing\n"), 0o644)
	if _, err := checkConfig(); err == nil {
		t.Error("a file with no known key must be an error")
	}
	os.WriteFile(path, []byte("editor = \"vi\"\n"), 0o644)
	if detail, err := checkConfig(); err != nil || !strings.Contains(detail, "editor=vi") {
		t.Errorf("file with editor: %q, %v", detail, err)
	}
}

func TestCheckEditor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, v := range []string{"FORESTRY_EDITOR", "VISUAL", "EDITOR"} {
		t.Setenv(v, "")
	}
	if _, err := checkEditor(); err == nil {
		t.Error("no editor must be an error")
	}
	t.Setenv("EDITOR", "no-such-editor-xyz")
	if _, err := checkEditor(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing editor: %v", err)
	}
	t.Setenv("EDITOR", "true -x")
	if detail, err := checkEditor(); err != nil || detail != "true -x" {
		t.Errorf("real editor: %q, %v", detail, err)
	}
}

func TestCheckWorktreesReportsMissingDirectory(t *testing.T) {
	stubCommand(t, func(name string, args ...string) (string, error) {
		return "worktree /nope/gone\nHEAD abc\nbranch refs/heads/x\n", nil
	})
	if _, err := checkWorktrees(); err == nil || !strings.Contains(err.Error(), "prune") {
		t.Errorf("got %v, want a prune hint", err)
	}
}

func TestCmdDoctorRejectsArguments(t *testing.T) {
	if err := cmdDoctor([]string{"x"}); err == nil {
		t.Error("doctor with arguments must fail")
	}
}
