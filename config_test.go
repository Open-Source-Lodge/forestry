package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir:", err)
	}
	tests := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"/absolute/path", "/absolute/path"},
		{"relative", "relative"},
		{"~other/path", "~other/path"},
	}
	for _, tt := range tests {
		got := expandHome(tt.in)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWorktreeRoot(t *testing.T) {
	t.Cleanup(func() { os.Unsetenv("FORESTRY_ROOT") })
	t.Setenv("HOME", t.TempDir()) // no ~/.forestry to interfere

	// Default: sibling directory named <repo>-worktrees.
	os.Unsetenv("FORESTRY_ROOT")
	got := worktreeRoot("/home/user/myrepo")
	want := "/home/user/myrepo-worktrees"
	if got != want {
		t.Errorf("worktreeRoot default = %q, want %q", got, want)
	}

	// FORESTRY_ROOT override.
	os.Setenv("FORESTRY_ROOT", "/mnt/worktrees")
	got = worktreeRoot("/home/user/myrepo")
	want = "/mnt/worktrees/myrepo"
	if got != want {
		t.Errorf("worktreeRoot FORESTRY_ROOT = %q, want %q", got, want)
	}
}

func TestConfigValue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	content := `# forestry settings
root = /mnt/worktrees
editor = "vim -p"
# another comment
empty =
`
	if err := os.WriteFile(filepath.Join(home, ".forestry"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		key  string
		want string
	}{
		{"root", "/mnt/worktrees"},
		{"editor", "vim -p"},
		{"empty", ""},
		{"missing", ""},
	}
	for _, tt := range tests {
		got := configValue(tt.key)
		if got != tt.want {
			t.Errorf("configValue(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestDeleteBranchOnRemove(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubCommand(t, func(name string, args ...string) (string, error) {
		return "worktree " + repo, nil
	})
	write := func(content string) {
		if err := os.WriteFile(filepath.Join(repo, ".forestry"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if deleteBranchOnRemove() {
		t.Error("no .forestry file should mean no branch deletion")
	}
	write("delete_branch = false\n")
	if deleteBranchOnRemove() {
		t.Error("delete_branch = false should mean no branch deletion")
	}
	write("# keep branches?\ndelete_branch = true\n")
	if !deleteBranchOnRemove() {
		t.Error("delete_branch = true should mean branch deletion")
	}

	// ~/.forestry applies when the repo has nothing to say, and only then.
	if err := os.WriteFile(filepath.Join(home, ".forestry"), []byte("delete_branch = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	write("delete_branch = false\n")
	if deleteBranchOnRemove() {
		t.Error("the repo .forestry should win over ~/.forestry")
	}
	if err := os.Remove(filepath.Join(repo, ".forestry")); err != nil {
		t.Fatal(err)
	}
	if !deleteBranchOnRemove() {
		t.Error("~/.forestry should apply with no repo .forestry")
	}
}

func TestEditorCommand(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("FORESTRY_EDITOR")
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")
	})
	os.Unsetenv("FORESTRY_EDITOR")
	os.Unsetenv("VISUAL")
	os.Unsetenv("EDITOR")
	// Point home at an empty dir so no ~/.forestry exists.
	t.Setenv("HOME", t.TempDir())

	if got := editorCommand(); got != nil {
		t.Errorf("expected nil when no editor set, got %v", got)
	}

	os.Setenv("EDITOR", "nano")
	got := editorCommand()
	if len(got) != 1 || got[0] != "nano" {
		t.Errorf("editorCommand with EDITOR=nano = %v, want [nano]", got)
	}

	os.Setenv("VISUAL", "code --wait")
	got = editorCommand()
	if len(got) != 2 || got[0] != "code" || got[1] != "--wait" {
		t.Errorf("editorCommand with VISUAL='code --wait' = %v", got)
	}

	os.Setenv("FORESTRY_EDITOR", "nvim")
	got = editorCommand()
	if len(got) != 1 || got[0] != "nvim" {
		t.Errorf("FORESTRY_EDITOR should take precedence, got %v", got)
	}
}
