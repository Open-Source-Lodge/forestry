package main

import (
	"testing"
)

func TestShortSHA(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"abc1234", "abc1234"},
		{"abc12345", "abc12345"},
		{"abc123456789", "abc12345"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := shortSHA(tt.in); got != tt.want {
			t.Errorf("shortSHA(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWorktreeNameAndRef(t *testing.T) {
	tests := []struct {
		wt   Worktree
		name string
		ref  string
	}{
		{
			wt:   Worktree{Path: "/home/user/project", Branch: "main"},
			name: "project",
			ref:  "main",
		},
		{
			wt:   Worktree{Path: "/home/user/project-worktrees/feat-login", Branch: "feat/login"},
			name: "feat-login",
			ref:  "feat/login",
		},
		{
			wt:   Worktree{Path: "/home/user/project", Head: "abcdef1234567890", Detached: true},
			name: "project",
			ref:  "detached at abcdef12",
		},
		{
			wt:   Worktree{Path: "/home/user/project"},
			name: "project",
			ref:  "-",
		},
	}
	for _, tt := range tests {
		if got := tt.wt.Name(); got != tt.name {
			t.Errorf("Name() = %q, want %q", got, tt.name)
		}
		if got := tt.wt.Ref(); got != tt.ref {
			t.Errorf("Ref() = %q, want %q", got, tt.ref)
		}
	}
}

func TestParseWorktrees(t *testing.T) {
	// Simulate parsing the porcelain output by calling worktrees() with a stub.
	// We test the parsing logic directly by calling the same parsing code path
	// that worktrees() uses, via a helper that accepts output as a string.
	input := `worktree /home/user/project
HEAD abc1234
branch refs/heads/main

worktree /home/user/project-worktrees/feat-login
HEAD def5678
branch refs/heads/feat/login

worktree /home/user/project-worktrees/detached-wt
HEAD deadbeef
detached

worktree /home/user/project-worktrees/locked-wt
HEAD cafebabe
branch refs/heads/locked-branch
locked
`
	list := parseWorktreeOutput(input)

	if len(list) != 4 {
		t.Fatalf("expected 4 worktrees, got %d", len(list))
	}

	// First entry is always the main worktree.
	if !list[0].Main {
		t.Error("first worktree should be main")
	}
	if list[0].Branch != "main" {
		t.Errorf("first worktree branch = %q, want %q", list[0].Branch, "main")
	}

	if list[1].Branch != "feat/login" {
		t.Errorf("second worktree branch = %q, want %q", list[1].Branch, "feat/login")
	}
	if list[1].Main {
		t.Error("second worktree should not be main")
	}

	if !list[2].Detached {
		t.Error("third worktree should be detached")
	}
	if list[2].Head != "deadbeef" {
		t.Errorf("third worktree HEAD = %q, want %q", list[2].Head, "deadbeef")
	}

	if !list[3].Locked {
		t.Error("fourth worktree should be locked")
	}
}
