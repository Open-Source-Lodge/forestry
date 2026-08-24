package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel returns a model populated with fake rows for testing.
func newTestModel(rows []row) model {
	sp := spinner.New()
	return model{
		repo:    "/tmp/testrepo",
		rows:    rows,
		spinner: sp,
	}
}

func fakeRows() []row {
	return []row{
		{wt: Worktree{Path: "/tmp/main", Branch: "main", Main: true}},
		{wt: Worktree{Path: "/tmp/feat-a", Branch: "feat/a"}},
		{wt: Worktree{Path: "/tmp/feat-b", Branch: "feat/b"}},
	}
}

func sendKey(m model, key string) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return next.(model), cmd
}

func sendSpecialKey(m model, t tea.KeyType) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: t})
	return next.(model), cmd
}

func TestCursorMovement(t *testing.T) {
	m := newTestModel(fakeRows())

	// Move down.
	m, _ = sendKey(m, "j")
	if m.cursor != 1 {
		t.Errorf("after j cursor = %d, want 1", m.cursor)
	}
	m, _ = sendKey(m, "j")
	if m.cursor != 2 {
		t.Errorf("after j cursor = %d, want 2", m.cursor)
	}
	// Cannot go past the last row.
	m, _ = sendKey(m, "j")
	if m.cursor != 2 {
		t.Errorf("cursor should clamp at 2, got %d", m.cursor)
	}

	// Move up.
	m, _ = sendKey(m, "k")
	if m.cursor != 1 {
		t.Errorf("after k cursor = %d, want 1", m.cursor)
	}

	// Jump to end / home.
	m, _ = sendKey(m, "G")
	if m.cursor != 2 {
		t.Errorf("G should jump to last row, got %d", m.cursor)
	}
	m, _ = sendKey(m, "g")
	if m.cursor != 0 {
		t.Errorf("g should jump to first row, got %d", m.cursor)
	}
}

func TestArrowKeyCursorMovement(t *testing.T) {
	m := newTestModel(fakeRows())
	m, _ = sendSpecialKey(m, tea.KeyDown)
	if m.cursor != 1 {
		t.Errorf("down cursor = %d, want 1", m.cursor)
	}
	m, _ = sendSpecialKey(m, tea.KeyUp)
	if m.cursor != 0 {
		t.Errorf("up cursor = %d, want 0", m.cursor)
	}
}

func TestModeNewAndCancel(t *testing.T) {
	m := newTestModel(fakeRows())
	if m.mode != modeList {
		t.Fatal("initial mode should be modeList")
	}

	m, _ = sendKey(m, "n")
	if m.mode != modeNew {
		t.Errorf("after n mode = %d, want modeNew(%d)", m.mode, modeNew)
	}

	// Escape should return to list.
	m, _ = sendSpecialKey(m, tea.KeyEsc)
	if m.mode != modeList {
		t.Errorf("after esc mode = %d, want modeList(%d)", m.mode, modeList)
	}
}

func TestModeRemoveOnMain(t *testing.T) {
	m := newTestModel(fakeRows())
	// cursor is on row 0 which is the main worktree — pressing d should show error.
	m, _ = sendKey(m, "d")
	if m.mode == modeRemove {
		t.Error("should not enter modeRemove for main worktree")
	}
	if !m.msgErr {
		t.Error("expected an error message for removing main worktree")
	}
}

func TestModeRemoveAndCancelOnNonMain(t *testing.T) {
	m := newTestModel(fakeRows())
	// Move to feat/a.
	m, _ = sendKey(m, "j")
	m, _ = sendKey(m, "d")
	if m.mode != modeRemove {
		t.Errorf("after d on non-main mode = %d, want modeRemove(%d)", m.mode, modeRemove)
	}
	// Cancel.
	m, _ = sendSpecialKey(m, tea.KeyEsc)
	if m.mode != modeList {
		t.Errorf("after esc mode = %d, want modeList", m.mode)
	}
}

func TestQuit(t *testing.T) {
	m := newTestModel(fakeRows())
	_, cmd := sendKey(m, "q")
	if cmd == nil {
		t.Fatal("q should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q command produced %T, want tea.QuitMsg", msg)
	}
}

func TestRowsMsg(t *testing.T) {
	m := newTestModel(nil)
	rows := fakeRows()
	next, _ := m.Update(rowsMsg{rows: rows, current: "/tmp/main"})
	m = next.(model)
	if len(m.rows) != 3 {
		t.Errorf("rows count = %d, want 3", len(m.rows))
	}
	if m.current != "/tmp/main" {
		t.Errorf("current = %q, want /tmp/main", m.current)
	}
}

func TestDoneMsg(t *testing.T) {
	m := newTestModel(fakeRows())
	m.busy = "doing something"

	next, cmd := m.Update(doneMsg{text: "done!", err: nil})
	m = next.(model)
	if m.busy != "" {
		t.Errorf("busy should be cleared after doneMsg, got %q", m.busy)
	}
	if m.msg != "done!" {
		t.Errorf("msg = %q, want %q", m.msg, "done!")
	}
	if cmd == nil {
		t.Error("doneMsg should schedule a loadRows command")
	}
}

func TestDoneMsgWithError(t *testing.T) {
	m := newTestModel(fakeRows())
	next, _ := m.Update(doneMsg{err: errors.New("something went wrong")})
	m = next.(model)
	if !m.msgErr {
		t.Error("msgErr should be true after error doneMsg")
	}
	if m.msg != "something went wrong" {
		t.Errorf("msg = %q, want %q", m.msg, "something went wrong")
	}
}

func TestSelected(t *testing.T) {
	m := newTestModel(fakeRows())
	wt, ok := m.selected()
	if !ok {
		t.Fatal("selected() should return ok with rows")
	}
	if wt.Branch != "main" {
		t.Errorf("selected branch = %q, want main", wt.Branch)
	}

	empty := newTestModel(nil)
	_, ok = empty.selected()
	if ok {
		t.Error("selected() should return !ok with no rows")
	}
}

func TestSetMsg(t *testing.T) {
	m := newTestModel(nil)
	m.setMsg("hello", nil)
	if m.msg != "hello" || m.msgErr {
		t.Error("setMsg with nil error should set msg and clear msgErr")
	}
	m.setMsg("", errors.New("bad"))
	if m.msg != "bad" || !m.msgErr {
		t.Errorf("setMsg with error should set msgErr, got msg=%q msgErr=%v", m.msg, m.msgErr)
	}
}

func TestWantCursorPositioning(t *testing.T) {
	m := newTestModel(nil)
	m.want = "/tmp/feat-b"
	rows := fakeRows()
	next, _ := m.Update(rowsMsg{rows: rows, current: ""})
	m = next.(model)
	if m.cursor != 2 {
		t.Errorf("cursor should be 2 (feat-b), got %d", m.cursor)
	}
	if m.want != "" {
		t.Error("want should be cleared after positioning")
	}
}

func TestCursorClampOnShrink(t *testing.T) {
	rows := fakeRows()
	m := newTestModel(rows)
	m.cursor = 2

	// Return fewer rows — cursor should clamp.
	fewer := fakeRows()[:1]
	next, _ := m.Update(rowsMsg{rows: fewer, current: ""})
	m = next.(model)
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0 after shrink, got %d", m.cursor)
	}
}

func TestNewInputTabCycles(t *testing.T) {
	m := newTestModel(fakeRows())
	m, _ = sendKey(m, "n")
	if m.mode != modeNew {
		t.Fatal("expected modeNew")
	}
	// Initial focus is 0.
	if m.focus != 0 {
		t.Errorf("initial focus = %d, want 0", m.focus)
	}
	// Tab should advance to next input.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(model)
	if m.focus != 1 {
		t.Errorf("after tab focus = %d, want 1", m.focus)
	}
	// Another tab wraps around.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(model)
	if m.focus != 0 {
		t.Errorf("after second tab focus = %d, want 0", m.focus)
	}
}

// Y removes the worktree and then deletes its local branch.
func TestRemoveCmdDeletesBranch(t *testing.T) {
	var ran []string
	stubCommand(t, func(name string, args ...string) (string, error) {
		ran = append(ran, strings.Join(args, " "))
		return "", nil
	})
	msg := removeCmd(Worktree{Path: "/x/wt", Branch: "feat"}, false, true)().(doneMsg)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	joined := strings.Join(ran, "\n")
	if !strings.Contains(joined, "worktree remove /x/wt") || !strings.Contains(joined, "branch -D feat") {
		t.Errorf("wrong git calls:\n%s", joined)
	}
}
