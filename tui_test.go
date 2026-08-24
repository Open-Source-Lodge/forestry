package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// stubGit makes each git call return out, or fail when fail is set.
func stubGit(t *testing.T, out string, fail bool) {
	t.Helper()
	stubCommand(t, func(string, ...string) (string, error) {
		if fail {
			return "", errors.New("stub failure")
		}
		return out, nil
	})
}

func TestRemoveKeysRunTheCorrectCommand(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"y", "worktree remove /tmp/feat-a"},
		{"f", "worktree remove /tmp/feat-a --force"},
		{"Y", "branch -D feat/a"},
	}
	for _, tt := range tests {
		var ran []string
		stubCommand(t, func(name string, args ...string) (string, error) {
			ran = append(ran, strings.Join(args, " "))
			return "", nil
		})
		m := newTestModel(fakeRows())
		m, _ = sendKey(m, "j")
		m, _ = sendKey(m, "d")
		m, cmd := sendKey(m, tt.key)
		if m.mode != modeList || m.busy == "" || m.busyPath != "/tmp/feat-a" {
			t.Errorf("%s: mode=%d busy=%q busyPath=%q", tt.key, m.mode, m.busy, m.busyPath)
		}
		// The batch holds the remove command and the spinner tick. Run each one.
		for _, msg := range []tea.Msg{cmd()} {
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					c()
				}
			}
		}
		if joined := strings.Join(ran, "\n"); !strings.Contains(joined, tt.want) {
			t.Errorf("%s: git calls:\n%s\nwant %q", tt.key, joined, tt.want)
		}
	}
}

func TestRemoveModeWithNoRowsReturnsToList(t *testing.T) {
	m := newTestModel(nil)
	m.mode = modeRemove
	m, _ = sendKey(m, "y")
	if m.mode != modeList {
		t.Errorf("mode = %d, want modeList", m.mode)
	}
}

func TestListKeysWhileBusy(t *testing.T) {
	m := newTestModel(fakeRows())
	m.busy = "working"
	m, cmd := sendKey(m, "j")
	if m.cursor != 0 || cmd != nil {
		t.Error("keys must do nothing while busy")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c must quit while busy")
	}
}

func TestEnterOpensShellPath(t *testing.T) {
	m := newTestModel(fakeRows())
	m, _ = sendKey(m, "j")
	m, cmd := sendSpecialKey(m, tea.KeyEnter)
	if m.openPath != "/tmp/feat-a" || cmd == nil {
		t.Errorf("openPath = %q, want /tmp/feat-a with a quit command", m.openPath)
	}
}

func TestOpenPRKeyWithoutPR(t *testing.T) {
	m := newTestModel(fakeRows())
	m, cmd := sendKey(m, "p")
	if !m.msgErr || cmd != nil {
		t.Errorf("p without a pull request: msgErr=%v cmd=%v", m.msgErr, cmd)
	}
	m.prs = map[string]PR{"main": {Number: 7, State: "open"}}
	m, cmd = sendKey(m, "p")
	if m.msgErr || cmd == nil {
		t.Error("p with a pull request must run a command")
	}
}

func TestRefreshKeyClearsMessage(t *testing.T) {
	m := newTestModel(fakeRows())
	m.msg = "old"
	m, cmd := sendKey(m, "r")
	if m.msg != "" || cmd == nil {
		t.Errorf("r: msg=%q cmd=%v", m.msg, cmd)
	}
}

func TestNewModeEnterCreates(t *testing.T) {
	m := newTestModel(fakeRows())
	m, _ = sendKey(m, "n")
	// Enter with no name does nothing.
	m, cmd := sendSpecialKey(m, tea.KeyEnter)
	if m.mode != modeNew || cmd != nil {
		t.Error("enter with an empty name must stay in modeNew")
	}
	m, _ = sendKey(m, "x")
	m, cmd = sendSpecialKey(m, tea.KeyEnter)
	if m.mode != modeList || m.busy != "creating x" || cmd == nil {
		t.Errorf("enter with a name: mode=%d busy=%q", m.mode, m.busy)
	}
}

func fakePRs(n int) []openPR {
	list := make([]openPR, n)
	for i := range list {
		list[i] = openPR{Number: i + 1, Title: "pr"}
	}
	return list
}

func TestPRPickerNavigation(t *testing.T) {
	m := newTestModel(fakeRows())
	m, _ = sendKey(m, "P")
	if m.mode != modePR {
		t.Fatal("expected modePR")
	}
	next, _ := m.Update(openPRsMsg{list: fakePRs(20)})
	m = next.(model)

	m, _ = sendSpecialKey(m, tea.KeyDown)
	m, _ = sendSpecialKey(m, tea.KeyRight)
	if m.prCursor != 1+prPageSize {
		t.Errorf("after down, right: prCursor = %d", m.prCursor)
	}
	m, _ = sendSpecialKey(m, tea.KeyRight)
	m, _ = sendSpecialKey(m, tea.KeyRight)
	if m.prCursor != 19 {
		t.Errorf("right must clamp at the last pull request, got %d", m.prCursor)
	}
	m, _ = sendSpecialKey(m, tea.KeyLeft)
	m, _ = sendSpecialKey(m, tea.KeyUp)
	if m.prCursor != 10 {
		t.Errorf("after left, up: prCursor = %d, want 10", m.prCursor)
	}

	// Enter uses the selected pull request when the input is empty.
	m, _ = sendSpecialKey(m, tea.KeyEnter)
	if m.mode != modeList || m.busy != "checking out PR 11" {
		t.Errorf("enter: mode=%d busy=%q", m.mode, m.busy)
	}
}

func TestPRPickerTypedNumberWins(t *testing.T) {
	m := newTestModel(fakeRows())
	m, _ = sendKey(m, "P")
	next, _ := m.Update(openPRsMsg{list: fakePRs(3)})
	m = next.(model)
	m, _ = sendKey(m, "9")
	m, _ = sendSpecialKey(m, tea.KeyEnter)
	if m.busy != "checking out PR 9" {
		t.Errorf("busy = %q, want the typed number", m.busy)
	}
}

func TestPRPickerEnterWithNothing(t *testing.T) {
	m := newTestModel(fakeRows())
	m, _ = sendKey(m, "P")
	next, _ := m.Update(openPRsMsg{err: errors.New("offline")})
	m = next.(model)
	if m.prErr != "offline" {
		t.Errorf("prErr = %q", m.prErr)
	}
	m, _ = sendSpecialKey(m, tea.KeyEnter)
	if m.mode != modePR {
		t.Error("enter with no list and no number must stay in modePR")
	}
	m, _ = sendSpecialKey(m, tea.KeyEsc)
	if m.mode != modeList {
		t.Error("esc must return to the list")
	}
}

func TestSpinnerTickOnlyWhileBusy(t *testing.T) {
	m := newTestModel(fakeRows())
	if _, cmd := m.Update(spinner.TickMsg{}); cmd != nil {
		t.Error("idle model must not keep the spinner running")
	}
}

func TestPrsMsgAndWindowSize(t *testing.T) {
	m := newTestModel(fakeRows())
	next, _ := m.Update(prsMsg{"feat/a": {Number: 3, State: "open"}})
	next, _ = next.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	m = next.(model)
	if m.prs["feat/a"].Number != 3 || m.width != 30 {
		t.Errorf("prs=%v width=%d", m.prs, m.width)
	}
}

func TestListViewShowsEachRow(t *testing.T) {
	stubGit(t, "", false)
	m := newTestModel(fakeRows())
	m.current = "/tmp/feat-a"
	m.cursor = 1
	m.busyPath = "/tmp/feat-b"
	m.rows[1].status = "dirty"
	m.prs = map[string]PR{"main": {Number: 1, State: "merged"}, "feat/a": {State: "closed", Number: 2}}
	out := m.View()
	for _, want := range []string{"forestry", "testrepo", "main", "feat-a", "feat-b", "merged #1", "closed #2", "❯", "•", listHelp} {
		if !strings.Contains(out, want) {
			t.Errorf("view lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(newTestModel(nil).View(), "no worktrees") {
		t.Error("empty list must say so")
	}
}

func TestViewPerMode(t *testing.T) {
	stubGit(t, " M file", false) // every worktree is dirty
	m := newTestModel(fakeRows())
	m.cursor = 1

	m.mode, m.inputs = modeNew, newInputs()
	if out := m.View(); !strings.Contains(out, "New worktree") || !strings.Contains(out, newHelp) {
		t.Errorf("new view:\n%s", out)
	}

	m.mode, m.inputs = modePR, prInputs()
	if out := m.View(); !strings.Contains(out, "loading open pull requests") {
		t.Errorf("pr view while loading:\n%s", out)
	}
	m.prList = []openPR{}
	if out := m.View(); !strings.Contains(out, "no open pull requests") {
		t.Errorf("pr view with no pull requests:\n%s", out)
	}
	m.prErr = "boom"
	if out := m.View(); !strings.Contains(out, "boom") {
		t.Errorf("pr view with error:\n%s", out)
	}
	m.prErr, m.prList, m.prCursor = "", fakePRs(10), 9
	out := m.View()
	if !strings.Contains(out, "page 2/2 · 10 open") || strings.Contains(out, "#1 ") {
		t.Errorf("pr view page 2:\n%s", out)
	}

	m.mode = modeRemove
	out = m.View()
	if !strings.Contains(out, "Remove feat-a?") || !strings.Contains(out, "needs force") {
		t.Errorf("remove view:\n%s", out)
	}
	m.cursor = 99
	if strings.Contains(m.View(), "Remove") {
		t.Error("remove view with no selection must be empty")
	}
}

func TestMsgView(t *testing.T) {
	m := newTestModel(fakeRows())
	if got := m.msgView(); got != "\n" {
		t.Errorf("no message: %q", got)
	}
	m.busy = "working"
	if !strings.Contains(m.msgView(), "working…") {
		t.Error("busy message missing")
	}
	m.busy = ""
	m.setMsg("", errors.New("bad"))
	if !strings.Contains(m.msgView(), "bad") {
		t.Error("error message missing")
	}
}

func TestClipCutsLongLines(t *testing.T) {
	m := newTestModel(nil)
	long := strings.Repeat("x", 50)
	if got := m.clip(long); got != long {
		t.Error("no width set: clip must not change the text")
	}
	m.width = 10
	if got := m.clip(long); len(got) != 10 {
		t.Errorf("clip to 10: got %d chars", len(got))
	}
}

func TestStyles(t *testing.T) {
	r := func(s lipgloss.Style) string { return s.Render("x") }
	if r(statusStyle("main, dirty")) != r(dirtyStyle) || r(statusStyle("clean")) != r(dimStyle) {
		t.Error("statusStyle")
	}
	if r(prStyle("merged")) != r(mergedStyle) || r(prStyle("open")) != r(okStyle) ||
		r(prStyle("closed")) != r(errStyle) || r(prStyle("")) != r(dimStyle) {
		t.Error("prStyle")
	}
}

func TestTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := tilde(home + "/x"); got != "~/x" {
		t.Errorf("tilde(home/x) = %q", got)
	}
	if got := tilde("/other/x"); got != "/other/x" {
		t.Errorf("tilde(/other/x) = %q", got)
	}
}

func TestCreateCmdsReportErrors(t *testing.T) {
	stubGit(t, "", true)
	if msg := createCmd("x", "")().(doneMsg); msg.err == nil {
		t.Error("createCmd must report the git failure")
	}
	if msg := createFromPRCmd("1")().(doneMsg); msg.err == nil {
		t.Error("createFromPRCmd must report the failure")
	}
	if msg := removeCmd(Worktree{Path: "/x"}, false, false)().(doneMsg); msg.err == nil {
		t.Error("removeCmd must report the failure")
	}
	if msg := loadRows().(doneMsg); msg.err == nil {
		t.Error("loadRows must report the failure")
	}
}

func TestEditorCmdWithoutEditor(t *testing.T) {
	for _, v := range []string{"FORESTRY_EDITOR", "VISUAL", "EDITOR"} {
		t.Setenv(v, "")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if msg := editorCmd("/x")().(doneMsg); msg.err == nil {
		t.Error("no editor must be an error")
	}
	t.Setenv("FORESTRY_EDITOR", "true")
	if editorCmd("/x") == nil {
		t.Error("an editor must give a command")
	}
}
