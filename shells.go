package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Forestry records each shell that `enter` opens in ~/.forestry-shells. Each
// line holds one shell: pid, tty, tmux pane, worktree path. The status column
// reads the file to show which worktrees have an open shell. The `s` key moves
// the focus to the terminal of that shell, when the terminal permits it.

type shell struct {
	pid  int
	tty  string // the terminal device, for example ttys004 or pts/1
	pane string // the tmux pane, when forestry ran inside tmux
	path string // the worktree
}

func shellsPath() string { return expandHome("~/.forestry-shells") }

// lockShells takes an exclusive lock on the registry file. The lock makes
// sure that a rewrite does not erase a record that a different process
// appends at the same time. The returned function releases the lock.
func lockShells() (func(), error) {
	f, err := os.OpenFile(shellsPath(), os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { f.Close() }, nil
}

// registerShell records the shell with pid that runs in the worktree at path.
func registerShell(pid int, path string) {
	// The shell shares the terminal with the forestry process that starts it.
	tty, _ := command("ps", "-o", "tty=", "-p", strconv.Itoa(os.Getpid()))
	unlock, err := lockShells()
	if err != nil {
		return
	}
	defer unlock()
	f, err := os.OpenFile(shellsPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	// ponytail: flock serializes each append and each rewrite; openShells
	// prunes what a crash leaves behind.
	fmt.Fprintf(f, "%d\t%s\t%s\t%s\n", pid, tty, os.Getenv("TMUX_PANE"), path)
	f.Close()
}

// unregisterShell removes the record of the shell with pid.
func unregisterShell(pid int) {
	unlock, err := lockShells()
	if err != nil {
		return
	}
	defer unlock()
	var keep []shell
	for _, s := range readShells() {
		if s.pid != pid {
			keep = append(keep, s)
		}
	}
	writeShells(keep)
}

// openShells is the set of live shells, keyed by worktree path. It removes the
// records of shells that no longer run.
func openShells() map[string][]shell {
	// Without the lock, forestry still reads the file, but does not prune it.
	unlock, lockErr := lockShells()
	if lockErr == nil {
		defer unlock()
	}
	all := readShells()
	var live []shell
	for _, s := range all {
		if alive(s.pid) {
			live = append(live, s)
		}
	}
	if lockErr == nil && len(live) != len(all) {
		writeShells(live)
	}
	byPath := make(map[string][]shell)
	for _, s := range live {
		byPath[s.path] = append(byPath[s.path], s)
	}
	return byPath
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func readShells() []shell {
	data, err := os.ReadFile(shellsPath())
	if err != nil {
		return nil
	}
	var list []shell
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		list = append(list, shell{pid: pid, tty: parts[1], pane: parts[2], path: parts[3]})
	}
	return list
}

func writeShells(list []shell) {
	var b strings.Builder
	for _, s := range list {
		fmt.Fprintf(&b, "%d\t%s\t%s\t%s\n", s.pid, s.tty, s.pane, s.path)
	}
	os.WriteFile(shellsPath(), []byte(b.String()), 0o600)
}

// focusShell moves the focus to the terminal that holds the shell. For a
// shell in tmux, forestry switches the tmux client; on macOS, forestry
// selects the Terminal.app or iTerm2 tab by its tty. On other systems,
// forestry can only say where the shell is.
func focusShell(s shell) error {
	if s.pane != "" {
		if _, err := command("tmux", "switch-client", "-t", s.pane); err == nil {
			return nil
		}
	}
	if runtime.GOOS == "darwin" {
		return focusTerminalApp("/dev/" + s.tty)
	}
	return fmt.Errorf("the shell is open on %s — forestry cannot move the focus on this system", s.tty)
}

// The scripts select the tab whose tty matches, and say "true" when one did.
const terminalScript = `set found to false
tell application "Terminal"
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is "%s" then
				set selected of t to true
				set index of w to 1
				set found to true
			end if
		end repeat
	end repeat
	if found then activate
end tell
return found`

const itermScript = `set found to false
tell application "iTerm2"
	repeat with aWindow in windows
		repeat with aTab in tabs of aWindow
			repeat with aSession in sessions of aTab
				if tty of aSession is "%s" then
					select aWindow
					select aTab
					select aSession
					set found to true
				end if
			end repeat
		end repeat
	end repeat
	if found then activate
end tell
return found`

// openShellTab opens a new terminal tab that runs a shell in the worktree at
// path. The tab runs `forestry shell`, so the shell registers itself. In tmux,
// the tab is a tmux window. Terminal.app has no AppleScript command for a new
// tab, so it gets a new window.
func openShellTab(path string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := "exec " + shq(self) + " shell " + shq(path)
	if os.Getenv("TMUX") != "" {
		_, err := command("tmux", "new-window", "-c", path, cmd)
		return err
	}
	if runtime.GOOS != "darwin" {
		return errors.New("forestry cannot open a tab in this terminal — it needs tmux, Terminal or iTerm2")
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "Apple_Terminal":
		_, err := command("osascript", "-e", fmt.Sprintf(terminalTabScript, asq(cmd)))
		return err
	case "iTerm.app":
		_, err := command("osascript", "-e", fmt.Sprintf(itermTabScript, asq(cmd)))
		return err
	}
	return errors.New("forestry cannot open a tab in this terminal — it needs tmux, Terminal or iTerm2")
}

const terminalTabScript = `tell application "Terminal"
	activate
	do script "%s"
end tell`

const itermTabScript = `tell application "iTerm2"
	activate
	if (count of windows) is 0 then
		create window with default profile
	else
		tell current window to create tab with default profile
	end if
	tell current session of current window to write text "%s"
end tell`

// shq quotes s as one shell word.
func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// asq escapes s for an AppleScript string literal.
func asq(s string) string { return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) }

func focusTerminalApp(tty string) error {
	scripts := []struct{ proc, script string }{
		{"Terminal", terminalScript},
		{"iTerm2", itermScript},
	}
	for _, a := range scripts {
		// pgrep makes sure that osascript does not start a terminal that
		// does not run.
		if _, err := command("pgrep", "-x", a.proc); err != nil {
			continue
		}
		out, err := command("osascript", "-e", fmt.Sprintf(a.script, asq(tty)))
		if err == nil && out == "true" {
			return nil
		}
	}
	return fmt.Errorf("the shell is open on %s, but no terminal window matches", strings.TrimPrefix(tty, "/dev/"))
}
