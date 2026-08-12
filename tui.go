package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	pickedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dirtyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	mergedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(7)
)

const (
	listHelp   = "↑↓ move · enter shell · e editor · n new · d remove · r refresh · q quit"
	newHelp    = "tab next field · enter create · esc cancel"
	removeHelp = "y remove · f force remove · esc cancel"
	busyHelp   = "working · ctrl+c quit"
)

type mode int

const (
	modeList mode = iota
	modeNew
	modeRemove
)

// row is a worktree together with the state the list renders for it.
type row struct {
	wt     Worktree
	status string
}

type rowsMsg struct {
	rows    []row
	current string
}

// prsMsg carries the pull request state of the branches, keyed by branch name.
// It arrives after the list, since looking it up may go over the network.
type prsMsg map[string]PR

// doneMsg reports the outcome of an action that ran outside the update loop.
type doneMsg struct {
	text string
	path string
	err  error
}

type model struct {
	repo    string
	rows    []row
	current string
	prs     map[string]PR
	cursor  int
	mode    mode
	inputs  []textinput.Model
	focus   int
	msg     string
	msgErr  bool
	// busy describes an action still running; busyPath marks its row.
	busy     string
	busyPath string
	spinner  spinner.Model
	// want is a path to move the cursor onto once the list reloads.
	want   string
	width  int
	height int
}

func tui() error {
	repo, err := mainWorktree()
	if err != nil {
		return err
	}
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(cursorStyle))
	// Start on the worktree we were launched from, if any.
	here, _ := git("rev-parse", "--show-toplevel")
	_, err = tea.NewProgram(model{repo: repo, spinner: sp, want: here}, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd { return loadRows }

func loadRows() tea.Msg {
	list, err := worktrees()
	if err != nil {
		return doneMsg{err: err}
	}
	rows := make([]row, len(list))
	for i, wt := range list {
		rows[i] = row{wt: wt, status: status(wt)}
	}
	current, _ := git("rev-parse", "--show-toplevel")
	return rowsMsg{rows: rows, current: current}
}

func loadPRs(rows []row) tea.Cmd {
	branches := make([]string, 0, len(rows))
	for _, r := range rows {
		branches = append(branches, r.wt.Branch)
	}
	return func() tea.Msg { return prsMsg(pullRequests(branches)) }
}

func createCmd(name, from string) tea.Cmd {
	return func() tea.Msg {
		path, err := createWorktree(name, from)
		if err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: "created " + filepath.Base(path), path: path}
	}
}

func removeCmd(wt Worktree, force bool) tea.Cmd {
	return func() tea.Msg {
		if err := removeWorktree(wt, force); err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: "removed " + wt.Name()}
	}
}

// shellCmd hands the terminal to an interactive shell rooted in the worktree,
// since a child process cannot change the directory of the shell that ran it.
func shellCmd(path string) tea.Cmd {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	c := exec.Command(sh)
	c.Dir = path
	c.Env = append(os.Environ(), "FORESTRY_WORKTREE="+path)
	return tea.ExecProcess(c, func(error) tea.Msg {
		return doneMsg{text: "left " + filepath.Base(path), path: path}
	})
}

// editorCmd opens the worktree in the configured editor. A terminal editor gets
// the screen for as long as it runs; a windowed one returns to the list at once.
func editorCmd(path string) tea.Cmd {
	argv := editorCommand()
	if len(argv) == 0 {
		return func() tea.Msg {
			return doneMsg{err: errors.New("no editor set — set FORESTRY_EDITOR, VISUAL or EDITOR")}
		}
	}
	args := append(append([]string{}, argv[1:]...), path)
	c := exec.Command(argv[0], args...)
	c.Dir = path
	c.Env = append(os.Environ(), "FORESTRY_WORKTREE="+path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: "opened " + filepath.Base(path) + " in " + argv[0], path: path}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case rowsMsg:
		m.rows, m.current = msg.rows, msg.current
		if m.want != "" {
			for i, r := range m.rows {
				if r.wt.Path == m.want {
					m.cursor = i
				}
			}
			m.want = ""
		}
		if m.cursor >= len(m.rows) {
			m.cursor = max(0, len(m.rows)-1)
		}
		return m, loadPRs(m.rows)

	case prsMsg:
		m.prs = msg

	case doneMsg:
		m.busy, m.busyPath = "", ""
		m.setMsg(msg.text, msg.err)
		if msg.err == nil {
			m.want = msg.path
		}
		return m, loadRows

	case spinner.TickMsg:
		if m.busy == "" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch m.mode {
		case modeNew:
			return m.updateNew(msg)
		case modeRemove:
			return m.updateRemove(msg)
		default:
			return m.updateList(msg)
		}
	}
	if m.mode == modeNew {
		return m.updateInputs(msg)
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.busy != "" {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j", "ctrl+n":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.rows) - 1
	case "r":
		m.setMsg("", nil)
		return m, loadRows
	case "enter", "o":
		if wt, ok := m.selected(); ok {
			return m, shellCmd(wt.Path)
		}
	case "e":
		if wt, ok := m.selected(); ok {
			m.setMsg("", nil)
			return m, editorCmd(wt.Path)
		}
	case "n":
		m.mode, m.focus, m.inputs = modeNew, 0, newInputs()
		m.setMsg("", nil)
		return m, textinput.Blink
	case "d", "x":
		wt, ok := m.selected()
		if !ok {
			break
		}
		if wt.Main {
			m.setMsg("", errors.New("refusing to remove the main worktree"))
			break
		}
		m.mode = modeRemove
		m.setMsg("", nil)
	}
	return m, nil
}

func (m model) updateNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeList
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.inputs[0].Value())
		if name == "" {
			return m, nil
		}
		m.mode = modeList
		return m.start("creating "+name, "", createCmd(name, strings.TrimSpace(m.inputs[1].Value())))
	case "tab", "down", "shift+tab", "up":
		if msg.String() == "tab" || msg.String() == "down" {
			m.focus = (m.focus + 1) % len(m.inputs)
		} else {
			m.focus = (m.focus - 1 + len(m.inputs)) % len(m.inputs)
		}
		for i := range m.inputs {
			if i == m.focus {
				m.inputs[i].Focus()
			} else {
				m.inputs[i].Blur()
			}
		}
		return m, textinput.Blink
	}
	return m.updateInputs(msg)
}

func (m model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m model) updateRemove(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	wt, ok := m.selected()
	if !ok {
		m.mode = modeList
		return m, nil
	}
	switch msg.String() {
	case "y", "enter":
		m.mode = modeList
		return m.start("removing "+wt.Name(), wt.Path, removeCmd(wt, false))
	case "f":
		m.mode = modeList
		return m.start("removing "+wt.Name(), wt.Path, removeCmd(wt, true))
	case "esc", "n", "q", "ctrl+c":
		m.mode = modeList
	}
	return m, nil
}

// start runs cmd while the list shows a spinner for path, if any.
func (m model) start(busy, path string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.busy, m.busyPath = busy, path
	m.setMsg("", nil)
	return m, tea.Batch(cmd, m.spinner.Tick)
}

func (m model) selected() (Worktree, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return Worktree{}, false
	}
	return m.rows[m.cursor].wt, true
}

func (m *model) setMsg(text string, err error) {
	if err != nil {
		m.msg, m.msgErr = err.Error(), true
		return
	}
	m.msg, m.msgErr = text, false
}

func newInputs() []textinput.Model {
	name := textinput.New()
	name.Prompt = ""
	name.Placeholder = "feat/login"
	name.Width = 40
	name.Focus()

	from := textinput.New()
	from.Prompt = ""
	from.Placeholder = "HEAD"
	from.Width = 40

	return []textinput.Model{name, from}
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render("forestry") + dimStyle.Render(" · "+filepath.Base(m.repo)) + "\n\n")
	b.WriteString(m.listView())
	b.WriteString("\n")
	switch m.mode {
	case modeNew:
		b.WriteString(m.newView())
	case modeRemove:
		b.WriteString(m.removeView())
	default:
		b.WriteString(m.msgView())
		help := listHelp
		if m.busy != "" {
			help = busyHelp
		}
		b.WriteString("  " + dimStyle.Render(help) + "\n")
	}
	return m.clip(b.String())
}

func (m model) listView() string {
	if len(m.rows) == 0 {
		return "  " + dimStyle.Render("no worktrees") + "\n"
	}
	var nameW, refW, prW int
	for _, r := range m.rows {
		nameW = max(nameW, lipgloss.Width(r.wt.Name()))
		refW = max(refW, lipgloss.Width(r.wt.Ref()))
		prW = max(prW, lipgloss.Width(m.pr(r).Label()))
	}
	name := lipgloss.NewStyle().Width(nameW + 2)
	ref := lipgloss.NewStyle().Width(refW + 2)
	stat := lipgloss.NewStyle().Width(9)
	// The pull request column stays away until there is something to put in it.
	pr := lipgloss.NewStyle()
	if prW > 0 {
		pr = pr.Width(prW + 2)
	}

	var b strings.Builder
	for i, r := range m.rows {
		cursor, label := "  ", r.wt.Name()
		if i == m.cursor {
			cursor, label = cursorStyle.Render("❯ "), pickedStyle.Render(label)
		}
		here := " "
		if r.wt.Path == m.current {
			here = cursorStyle.Render("•")
		}
		state := statusStyle(r.status).Render(r.status)
		if r.wt.Path == m.busyPath {
			state = m.spinner.View()
		}
		p := m.pr(r)
		b.WriteString(cursor + here + " " + name.Render(label) +
			ref.Render(dimStyle.Render(r.wt.Ref())) +
			stat.Render(state) +
			pr.Render(prStyle(p.State).Render(p.Label())) +
			dimStyle.Render(tilde(r.wt.Path)) + "\n")
	}
	return b.String()
}

// pr is the pull request of a row's branch, zero when it has none or the
// lookup has not landed yet.
func (m model) pr(r row) PR { return m.prs[r.wt.Branch] }

func statusStyle(s string) lipgloss.Style {
	if strings.Contains(s, "dirty") {
		return dirtyStyle
	}
	return dimStyle
}

func prStyle(state string) lipgloss.Style {
	switch state {
	case "merged":
		return mergedStyle
	case "open":
		return okStyle
	case "closed":
		return errStyle
	}
	return dimStyle
}

func (m model) newView() string {
	var b strings.Builder
	b.WriteString("  " + titleStyle.Render("New worktree") + "\n\n")
	b.WriteString("  " + labelStyle.Render("branch") + m.inputs[0].View() + "\n")
	b.WriteString("  " + labelStyle.Render("from") + m.inputs[1].View() + "\n\n")
	b.WriteString("  " + dimStyle.Render(newHelp) + "\n")
	return b.String()
}

func (m model) removeView() string {
	wt, ok := m.selected()
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString("  " + titleStyle.Render("Remove "+wt.Name()+"?") + "\n")
	b.WriteString("  " + dimStyle.Render(tilde(wt.Path)) + "\n\n")
	if isDirty(wt.Path) {
		b.WriteString("  " + dirtyStyle.Render("uncommitted changes — needs force") + "\n")
	}
	b.WriteString("  " + dimStyle.Render(removeHelp) + "\n")
	return b.String()
}

func (m model) msgView() string {
	if m.busy != "" {
		return "  " + m.spinner.View() + dimStyle.Render(m.busy+"…") + "\n"
	}
	if m.msg == "" {
		return "\n"
	}
	style := okStyle
	if m.msgErr {
		style = errStyle
	}
	return "  " + style.Render(m.msg) + "\n"
}

// clip keeps long lines from wrapping and pushing the layout around.
func (m model) clip(s string) string {
	if m.width == 0 {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(s)
}

func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(path, home+"/") {
		return path
	}
	return "~" + strings.TrimPrefix(path, home)
}
