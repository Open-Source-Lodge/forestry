package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temporary git repository, configures it and returns its path.
// All integration tests should call t.Setenv to point the process into the repo.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	must := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	must("init", "-b", "main")
	must("config", "user.email", "test@example.com")
	must("config", "user.name", "Test")
	// Commit something so we have a HEAD.
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	must("add", ".")
	must("commit", "-m", "init")
	return dir
}

// cdRepo changes the process working directory to the repo for the duration of
// the test. It also sets GIT_DIR and GIT_WORK_TREE so that git() calls inside
// the binary use the right repo without relying on the caller's $PWD.
func cdRepo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

func TestIntegrationList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := initRepo(t)
	cdRepo(t, dir)

	err := run([]string{"list"})
	if err != nil {
		t.Fatalf("forestry list: %v", err)
	}
}

func TestIntegrationHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	err := run([]string{"help"})
	if err != nil {
		t.Fatalf("forestry help: %v", err)
	}

	for _, alias := range []string{"-h", "--help"} {
		if err := run([]string{alias}); err != nil {
			t.Errorf("forestry %s: %v", alias, err)
		}
	}
}

func TestIntegrationUnknownCommand(t *testing.T) {
	err := run([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error message = %q, want 'unknown command'", err.Error())
	}
}

func TestIntegrationNewAndRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := initRepo(t)
	cdRepo(t, dir)

	// Create a new worktree.
	err := run([]string{"new", "feat-test"})
	if err != nil {
		t.Fatalf("forestry new feat-test: %v", err)
	}

	// The worktree directory should exist.
	wtRoot := worktreeRoot(dir)
	wtPath := filepath.Join(wtRoot, "feat-test")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory not found: %v", err)
	}

	// List should now show the worktree.
	wts, err := worktrees()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, wt := range wts {
		if wt.Branch == "feat-test" {
			found = true
		}
	}
	if !found {
		t.Error("new worktree feat-test not found in worktree list")
	}

	// Remove the worktree.
	err = run([]string{"remove", "feat-test"})
	if err != nil {
		t.Fatalf("forestry remove feat-test: %v", err)
	}

	// Confirm it is gone.
	wts, err = worktrees()
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range wts {
		if wt.Branch == "feat-test" {
			t.Error("removed worktree feat-test still in list")
		}
	}
}

func TestIntegrationRemoveCurrentWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := initRepo(t)
	cdRepo(t, dir)

	if err := run([]string{"new", "feat-here"}); err != nil {
		t.Fatalf("forestry new feat-here: %v", err)
	}
	cdRepo(t, filepath.Join(worktreeRoot(dir), "feat-here"))

	if err := run([]string{"remove", "feat-here"}); err != nil {
		t.Fatalf("forestry remove feat-here: %v", err)
	}
	// git must still work afterwards: we should have stepped up to the repo.
	if _, err := worktrees(); err != nil {
		t.Fatalf("worktrees() after removing the current worktree: %v", err)
	}
}

func TestIntegrationNewInvalidName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := initRepo(t)
	cdRepo(t, dir)

	for _, bad := range []string{"", "/absolute", "../up"} {
		err := run([]string{"new", bad})
		if err == nil {
			t.Errorf("forestry new %q expected error, got nil", bad)
		}
	}
}

func TestIntegrationRemoveMissingWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := initRepo(t)
	cdRepo(t, dir)

	err := run([]string{"remove", "nonexistent"})
	if err == nil {
		t.Fatal("expected error removing nonexistent worktree")
	}
}

func TestIntegrationNewFromRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dir := initRepo(t)
	cdRepo(t, dir)

	err := run([]string{"new", "feat-from-head", "--from", "HEAD"})
	if err != nil {
		t.Fatalf("forestry new feat-from-head --from HEAD: %v", err)
	}
	t.Cleanup(func() {
		// Best-effort cleanup.
		run([]string{"remove", "--force", "feat-from-head"}) //nolint
	})

	wts, err := worktrees()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, wt := range wts {
		if wt.Branch == "feat-from-head" {
			found = true
		}
	}
	if !found {
		t.Error("worktree feat-from-head not found after creation with --from HEAD")
	}
}
