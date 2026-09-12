package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is one entry of `git worktree list`.
type Worktree struct {
	Path   string
	Branch string
	Head   string
	Main   bool
	Locked bool
}

func (w Worktree) Name() string { return filepath.Base(w.Path) }

// Ref describes what the worktree has checked out.
func (w Worktree) Ref() string {
	switch {
	case w.Branch != "":
		return w.Branch
	case w.Head != "":
		return "detached at " + shortSHA(w.Head)
	default:
		return "-"
	}
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func git(args ...string) (string, error) {
	return command("git", args...)
}

// command is a var so tests can stand in for the real process.
var command = func(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func worktrees() ([]Worktree, error) {
	out, err := git("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeOutput(out), nil
}

func parseWorktreeOutput(out string) []Worktree {
	var list []Worktree
	var cur *Worktree
	for _, line := range strings.Split(out, "\n") {
		key, val, _ := strings.Cut(line, " ")
		if key == "worktree" {
			list = append(list, Worktree{Path: val})
			cur = &list[len(list)-1]
			continue
		}
		if cur == nil {
			continue
		}
		switch key {
		case "HEAD":
			cur.Head = val
		case "branch":
			cur.Branch = strings.TrimPrefix(val, "refs/heads/")
		case "locked":
			cur.Locked = true
		}
	}
	if len(list) > 0 {
		list[0].Main = true
	}
	return list
}

// mainWorktree is the original checkout, which git always lists first.
func mainWorktree() (string, error) {
	list, err := worktrees()
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", errors.New("not inside a git repository")
	}
	return list[0].Path, nil
}

// findWorktree resolves a worktree by directory name, or by path.
func findWorktree(name string) (Worktree, error) {
	list, err := worktrees()
	if err != nil {
		return Worktree{}, err
	}
	abs, _ := filepath.Abs(name)
	for _, w := range list {
		if w.Name() == name || w.Path == abs {
			return w, nil
		}
	}
	return Worktree{}, fmt.Errorf("no worktree named %q", name)
}

func branchExists(name string) bool {
	return refExists("refs/heads/" + name)
}

// ponytail: the remote is "origin", as it is in pr.go.
func remoteBranchExists(name string) bool {
	return refExists("refs/remotes/origin/" + name)
}

func refExists(ref string) bool {
	_, err := git("show-ref", "--verify", "--quiet", ref)
	return err == nil
}

// isDirty reports whether the worktree has uncommitted changes.
func isDirty(path string) bool {
	out, err := git("-C", path, "status", "--porcelain")
	return err == nil && out != ""
}
