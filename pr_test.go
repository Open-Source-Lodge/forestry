package main

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stubCommand replaces the process runner for one test.
func stubCommand(t *testing.T, run func(name string, args ...string) (string, error)) {
	t.Helper()
	real := command
	command = run
	t.Cleanup(func() { command = real })
}

// A branch with several pull requests keeps the loudest one.
func TestGhPullRequestsKeepsLoudest(t *testing.T) {
	stubCommand(t, func(string, ...string) (string, error) {
		return `[{"number":1,"state":"OPEN","isDraft":false,"headRefName":"feat"},
		         {"number":2,"state":"MERGED","isDraft":false,"headRefName":"feat"}]`, nil
	})

	prs, err := ghPullRequests()
	if err != nil {
		t.Fatal(err)
	}
	want := PR{Number: 2, State: "merged"}
	if prs["feat"] != want {
		t.Errorf("got %+v, want %+v", prs["feat"], want)
	}
}

func TestOpenPRCmd(t *testing.T) {
	var got []string
	stubCommand(t, func(name string, args ...string) (string, error) {
		got = append([]string{name}, args...)
		return "", nil
	})

	msg := openPRCmd(PR{Number: 42, State: "open"})().(doneMsg)
	if want := "gh pr view --web 42"; strings.Join(got, " ") != want {
		t.Errorf("ran %q, want %q", strings.Join(got, " "), want)
	}
	if msg.err != nil || !strings.Contains(msg.text, "#42") {
		t.Errorf("got %+v, want a message naming #42", msg)
	}

	stubCommand(t, func(string, ...string) (string, error) { return "", errors.New("no browser") })
	if msg := openPRCmd(PR{Number: 42})().(doneMsg); msg.err == nil {
		t.Errorf("got %+v, want the failure reported", msg)
	}
}

// "p" only has something to open when the branch carries a pull request.
func TestKeyPOpensPR(t *testing.T) {
	stubCommand(t, func(string, ...string) (string, error) {
		t.Error("no pull request linked, yet a command ran")
		return "", nil
	})

	m := model{
		rows: []row{{wt: Worktree{Path: "/wt/feat", Branch: "feat"}}},
		prs:  map[string]PR{},
	}
	next, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if cmd != nil {
		t.Error("no pull request linked, yet an action was started")
	}
	if got := next.(model); !got.msgErr || !strings.Contains(got.msg, "no pull request") {
		t.Errorf("got msg %q (err %v), want a no-pull-request error", got.msg, got.msgErr)
	}

	m.prs["feat"] = PR{Number: 7, State: "open"}
	if _, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); cmd == nil {
		t.Error("pull request #7 linked, but nothing was opened")
	}
}

func TestListHelpMentionsPR(t *testing.T) {
	if !strings.Contains(listHelp, "p open PR") {
		t.Errorf("list help does not mention the key: %q", listHelp)
	}
}

func TestPRLabel(t *testing.T) {
	tests := []struct {
		pr   PR
		want string
	}{
		{PR{Number: 0, State: ""}, ""},
		{PR{Number: 0, State: "merged"}, "merged"},
		{PR{Number: 42, State: "open"}, "open #42"},
		{PR{Number: 7, State: "draft"}, "draft #7"},
		{PR{Number: 1, State: "closed"}, "closed #1"},
	}
	for _, tt := range tests {
		if got := tt.pr.Label(); got != tt.want {
			t.Errorf("PR%+v.Label() = %q, want %q", tt.pr, got, tt.want)
		}
	}
}

func TestRank(t *testing.T) {
	if rank("merged") <= rank("open") {
		t.Error("merged should outrank open")
	}
	if rank("open") != rank("draft") {
		t.Error("open and draft should have equal rank")
	}
	if rank("open") <= rank("closed") {
		t.Error("open should outrank closed")
	}
	if rank("closed") != rank("unknown") {
		t.Error("closed and unknown state should have equal rank")
	}
}

// With no gh, the list falls back to the local ancestry check.
func TestPullRequestsFallsBackToMergedBranches(t *testing.T) {
	stubCommand(t, func(name string, args ...string) (string, error) {
		call := name + " " + strings.Join(args, " ")
		switch {
		case name == "gh":
			return "", errors.New("gh: not found")
		case strings.HasPrefix(call, "git symbolic-ref"):
			return "origin/main", nil
		case strings.Contains(call, "--is-ancestor done origin/main"):
			return "", nil
		}
		return "", errors.New("not an ancestor")
	})
	prs := pullRequests([]string{"done", "open", "main", ""})
	if prs["done"].State != "merged" || len(prs) != 1 {
		t.Errorf("got %v, want only done as merged", prs)
	}
}

func TestDefaultBranchFallsBackToLocalRefs(t *testing.T) {
	stubCommand(t, func(name string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify --quiet master" {
			return "", nil
		}
		return "", errors.New("no")
	})
	if got := defaultBranch(); got != "master" {
		t.Errorf("defaultBranch() = %q, want master", got)
	}
	stubCommand(t, func(string, ...string) (string, error) { return "", errors.New("no") })
	if got := defaultBranch(); got != "" {
		t.Errorf("defaultBranch() with no refs = %q, want empty", got)
	}
	if mergedBranches([]string{"x"}) != nil {
		t.Error("mergedBranches with no default branch must be nil")
	}
}
