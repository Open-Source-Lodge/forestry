package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	listHelp   = "↑↓ move · enter shell · e editor · n new · P from PR · d remove · p open PR · r refresh · q quit"
	newHelp    = "tab next field · enter create · esc cancel"
	prHelp     = "↑↓ pick · ←→ page · type a number · enter create · esc cancel"
	removeHelp = "y remove · Y remove + delete branch · f force remove · esc cancel"
	busyHelp   = "working · ctrl+c quit"
)

type mode int

// prPageSize is how many pull requests the picker shows at a time.
const prPageSize = 8

const (
	modeList mode = iota
	modeNew
	modePR
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

// openPRsMsg carries the pull requests the picker offers, or why there are none.
type openPRsMsg struct {
	list []openPR
	err  error
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
	// openPath asks tui() to open a shell there after Bubble Tea exits.
	openPath string
	mode     mode
	inputs   []textinput.Model
	focus    int
	msg      string
	msgErr   bool
	// busy describes an action still running; busyPath marks its row.
	busy     string
	busyPath string
	spinner  spinner.Model
	// picker state for modePR: the open pull requests, nil until they land.
	prList   []openPR
	prCursor int
	prErr    string
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
	mdl, err := tea.NewProgram(model{repo: repo, spinner: sp, want: here}, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	m, ok := mdl.(model)
	if !ok || m.openPath == "" {
		return nil
	}
	return runShell(m.openPath)
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

func loadOpenPRs() tea.Msg {
	list, err := openPRs()
	return openPRsMsg{list: list, err: err}
}

func createFromPRCmd(number string) tea.Cmd {
	return func() tea.Msg {
		path, err := createFromPR(number)
		if err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: "created " + filepath.Base(path), path: path}
	}
}

// removeCmd removes wt. With branch set, it then deletes the local branch too.
func removeCmd(wt Worktree, force, branch bool) tea.Cmd {
	return func() tea.Msg {
		if err := removeWorktree(wt, force); err != nil {
			return doneMsg{err: err}
		}
		if !branch || wt.Branch == "" {
			return doneMsg{text: "removed " + wt.Name()}
		}
		// ponytail: -D, because the user asked for the branch to go.
		if _, err := git("branch", "-D", wt.Branch); err != nil {
			return doneMsg{err: fmt.Errorf("removed %s, but: %w", wt.Name(), err)}
		}
		return doneMsg{text: "removed " + wt.Name() + " and branch " + wt.Branch}
	}
}

// runShell runs an interactive shell rooted in path.
func runShell(path string) error {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	c := exec.Command(sh)
	c.Dir = path
	c.Env = append(os.Environ(), "FORESTRY_WORKTREE="+path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
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

// openPRCmd opens the pull request URL in the default browser via the GitHub CLI.
func openPRCmd(pr PR) tea.Cmd {
	return func() tea.Msg {
		if _, err := command("gh", "pr", "view", "--web", fmt.Sprintf("%d", pr.Number)); err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: fmt.Sprintf("opened PR #%d in browser", pr.Number)}
	}
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

	case openPRsMsg:
		m.prList = msg.list
		if msg.err != nil {
			m.prErr = msg.err.Error()
		}

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
		case modePR:
			return m.updatePR(msg)
		case modeRemove:
			return m.updateRemove(msg)
		default:
			return m.updateList(msg)
		}
	}
	if m.mode == modeNew || m.mode == modePR {
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
			m.openPath = wt.Path
			return m, tea.Quit
		}
	case "e":
		if wt, ok := m.selected(); ok {
			m.setMsg("", nil)
			return m, editorCmd(wt.Path)
		}
	case "p":
		if wt, ok := m.selected(); ok {
			p := m.prs[wt.Branch]
			if p.Number == 0 {
				m.setMsg("", errors.New("no pull request linked to this worktree"))
				break
			}
			m.setMsg("", nil)
			return m, openPRCmd(p)
		}
	case "n":
		m.mode, m.focus, m.inputs = modeNew, 0, newInputs()
		m.setMsg("", nil)
		return m, textinput.Blink
	case "P":
		m.mode, m.focus, m.inputs = modePR, 0, prInputs()
		m.prList, m.prCursor, m.prErr = nil, 0, ""
		m.setMsg("", nil)
		return m, tea.Batch(textinput.Blink, loadOpenPRs)
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

// updatePR drives the picker. What you type wins over what is selected, so a
// number for a pull request the list does not offer still works.
func (m model) updatePR(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeList
		return m, nil
	case "up", "ctrl+p":
		m.prCursor = max(0, m.prCursor-1)
		return m, nil
	case "down", "ctrl+n":
		m.prCursor = min(len(m.prList)-1, m.prCursor+1)
		return m, nil
	case "left", "pgup":
		m.prCursor = max(0, m.prCursor-prPageSize)
		return m, nil
	case "right", "pgdown":
		m.prCursor = min(len(m.prList)-1, m.prCursor+prPageSize)
		return m, nil
	case "enter":
		number := strings.TrimSpace(m.inputs[0].Value())
		if number == "" {
			if m.prCursor < 0 || m.prCursor >= len(m.prList) {
				return m, nil
			}
			number = strconv.Itoa(m.prList[m.prCursor].Number)
		}
		m.mode = modeList
		return m.start("checking out PR "+number, "", createFromPRCmd(number))
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
		return m.start("removing "+wt.Name(), wt.Path, removeCmd(wt, false, false))
	case "Y":
		m.mode = modeList
		return m.start("removing "+wt.Name(), wt.Path, removeCmd(wt, false, true))
	case "f":
		m.mode = modeList
		return m.start("removing "+wt.Name(), wt.Path, removeCmd(wt, true, false))
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

func prInputs() []textinput.Model {
	number := textinput.New()
	number.Prompt = ""
	number.Placeholder = "123"
	number.Width = 40
	number.Focus()
	return []textinput.Model{number}
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render("forestry") + dimStyle.Render(" · "+filepath.Base(m.repo)) + "\n\n")
	b.WriteString(m.listView())
	b.WriteString("\n")
	switch m.mode {
	case modeNew:
		b.WriteString(m.newView())
	case modePR:
		b.WriteString(m.prView())
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

func (m model) prView() string {
	var b strings.Builder
	b.WriteString("  " + titleStyle.Render("Worktree from pull request") + "\n\n")
	b.WriteString(m.pickerView())
	b.WriteString("  " + labelStyle.Render("number") + m.inputs[0].View() + "\n\n")
	b.WriteString("  " + dimStyle.Render(prHelp) + "\n")
	return b.String()
}

// pickerView shows one page of open pull requests around the selection.
func (m model) pickerView() string {
	switch {
	case m.prErr != "":
		return "  " + dirtyStyle.Render("gh could not list pull requests: "+m.prErr) + "\n\n"
	case m.prList == nil:
		return "  " + dimStyle.Render("loading open pull requests…") + "\n\n"
	case len(m.prList) == 0:
		return "  " + dimStyle.Render("no open pull requests") + "\n\n"
	}
	page := m.prCursor / prPageSize
	start := page * prPageSize
	end := min(start+prPageSize, len(m.prList))

	var b strings.Builder
	for i := start; i < end; i++ {
		p := m.prList[i]
		cursor, title := "  ", p.Title
		if i == m.prCursor {
			cursor, title = cursorStyle.Render("❯ "), pickedStyle.Render(title)
		}
		b.WriteString(cursor + okStyle.Render(fmt.Sprintf("#%-5d", p.Number)) +
			dimStyle.Render(p.Date()) + "  " + title + "\n")
	}
	pages := (len(m.prList) + prPageSize - 1) / prPageSize
	b.WriteString("\n  " + dimStyle.Render(fmt.Sprintf("page %d/%d · %d open", page+1, pages, len(m.prList))) + "\n\n")
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
