package main

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// createFromPR asks gh for the head branch, fetches the pull request ref under
// that name, and checks it out in a worktree.
func TestCreateFromPRFetchesPullRef(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	var ran []string
	var fetched bool // the fetch creates the branch, as the real one does
	stubCommand(t, func(name string, args ...string) (string, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		ran = append(ran, line)
		switch {
		case name == "gh":
			return "feat/login", nil
		case strings.Contains(line, "show-ref"):
			if !fetched {
				return "", errors.New("not local yet")
			}
		case strings.Contains(line, "fetch"):
			fetched = true
		case strings.Contains(line, "worktree list"):
			return "worktree " + repo + "\nbranch refs/heads/main\n", nil
		}
		return "", nil
	})

	path, err := createFromPR("#42")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ran, "\n")
	if !strings.Contains(joined, "git fetch origin pull/42/head:feat/login") {
		t.Errorf("pull ref not fetched:\n%s", joined)
	}
	if !strings.Contains(joined, "worktree add "+repo+"-worktrees/feat-login feat/login") {
		t.Errorf("worktree not added on the pull request branch:\n%s", joined)
	}
	if !strings.HasSuffix(path, "feat-login") {
		t.Errorf("got path %q, want it to end in feat-login", path)
	}
}

// An existing local branch is checked out as it is, without a fetch over it.
func TestCreateFromPRKeepsLocalBranch(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	var ran []string
	stubCommand(t, func(name string, args ...string) (string, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		ran = append(ran, line)
		switch {
		case name == "gh":
			return "feat/login", nil
		case strings.Contains(line, "worktree list"):
			return "worktree " + repo + "\nbranch refs/heads/main\n", nil
		}
		return "", nil // show-ref succeeds → branch exists
	})

	if _, err := createFromPR("42"); err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(ran, "\n"); strings.Contains(joined, "fetch") {
		t.Errorf("fetched over an existing branch:\n%s", joined)
	}
}

func TestCreateFromPRRejectsNonNumbers(t *testing.T) {
	stubCommand(t, func(string, ...string) (string, error) {
		t.Error("nothing should run for an invalid number")
		return "", nil
	})
	for _, arg := range []string{"", "abc", "0", "-1", "1;rm"} {
		if _, err := createFromPR(arg); err == nil {
			t.Errorf("createFromPR(%q) was accepted", arg)
		}
	}
}

// The picker offers the open pull requests, most recently updated first.
func TestOpenPRsSortedByDate(t *testing.T) {
	var asked []string
	stubCommand(t, func(name string, args ...string) (string, error) {
		asked = append([]string{name}, args...)
		return `[{"number":1,"title":"old","headRefName":"a","updatedAt":"2026-01-01T10:00:00Z"},
		         {"number":9,"title":"new","headRefName":"b","updatedAt":"2026-08-14T10:00:00Z"},
		         {"number":5,"title":"mid","headRefName":"c","updatedAt":"2026-04-02T10:00:00Z"}]`, nil
	})

	list, err := openPRs()
	if err != nil {
		t.Fatal(err)
	}
	if q := strings.Join(asked, " "); !strings.Contains(q, "--state open") && !strings.Contains(q, "open") {
		t.Errorf("did not ask for open pull requests: %v", asked)
	}
	var got []int
	for _, p := range list {
		got = append(got, p.Number)
	}
	if want := []int{9, 5, 1}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v (newest first)", got, want)
	}
	if d := list[0].Date(); d != "2026-08-14" {
		t.Errorf("got date %q, want 2026-08-14", d)
	}
}

// enter on a selected row checks that pull request out; a typed number wins.
func TestPickerEnterPicksSelection(t *testing.T) {
	m := model{mode: modePR, inputs: prInputs(), prCursor: 1, prList: []openPR{
		{Number: 9}, {Number: 5}, {Number: 1},
	}}

	next, _ := m.updatePR(tea.KeyMsg{Type: tea.KeyEnter})
	if busy := next.(model).busy; busy != "checking out PR 5" {
		t.Errorf("got %q, want the selected pull request", busy)
	}

	m.inputs[0].SetValue("42")
	next, _ = m.updatePR(tea.KeyMsg{Type: tea.KeyEnter})
	if busy := next.(model).busy; busy != "checking out PR 42" {
		t.Errorf("got %q, want the typed number to win", busy)
	}
}

// Paging moves a page at a time and stops at both ends.
func TestPickerPaging(t *testing.T) {
	list := make([]openPR, prPageSize*2+3)
	m := model{mode: modePR, inputs: prInputs(), prList: list}

	next, _ := m.updatePR(tea.KeyMsg{Type: tea.KeyRight})
	if got := next.(model).prCursor; got != prPageSize {
		t.Errorf("got cursor %d, want %d", got, prPageSize)
	}
	m.prCursor = len(list) - 1
	next, _ = m.updatePR(tea.KeyMsg{Type: tea.KeyRight})
	if got := next.(model).prCursor; got != len(list)-1 {
		t.Errorf("paged past the end to %d", got)
	}
	m.prCursor = 0
	next, _ = m.updatePR(tea.KeyMsg{Type: tea.KeyLeft})
	if got := next.(model).prCursor; got != 0 {
		t.Errorf("paged before the start to %d", got)
	}

	// Every page is rendered, and the footer counts them.
	m.prCursor = prPageSize
	if view := m.prView(); !strings.Contains(view, "page 2/3") {
		t.Errorf("footer missing the page count:\n%s", view)
	}
}

// Without gh the picker says so and still takes a typed number.
func TestPickerWithoutGh(t *testing.T) {
	stubCommand(t, func(string, ...string) (string, error) { return "", errors.New("gh: not found") })

	msg := loadOpenPRs().(openPRsMsg)
	m := model{mode: modePR, inputs: prInputs()}
	updated, _ := m.Update(msg)
	m = updated.(model)
	if !strings.Contains(m.prView(), "gh: not found") {
		t.Errorf("the failure is not shown:\n%s", m.prView())
	}

	m.inputs[0].SetValue("42")
	next, _ := m.updatePR(tea.KeyMsg{Type: tea.KeyEnter})
	if busy := next.(model).busy; busy != "checking out PR 42" {
		t.Errorf("got %q, want a typed number to still work", busy)
	}
}
