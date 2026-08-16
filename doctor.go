package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Checks are what `forestry doctor` verifies. Every feature that leans on
// something outside the process — a binary on PATH, an environment variable, a
// directory it writes to, a network call — gets a line here, so a new feature
// means a new check. Return an error to fail; set warn when the feature only
// degrades (forestry still works, just with less).
var checks = []check{
	{name: "git", run: checkGit},
	{name: "repository", run: checkRepo},
	{name: "worktrees", run: checkWorktrees},
	{name: "worktree root", run: checkRoot},
	{name: "config", run: checkConfig, warn: true},
	{name: "github", run: checkGitHub, warn: true},
	{name: "merge fallback", run: checkFallback, warn: true},
	{name: "shell", run: checkShell, warn: true},
	{name: "editor", run: checkEditor, warn: true},
}

// check reports on one thing forestry needs. The detail is shown either way.
type check struct {
	name string
	run  func() (string, error)
	warn bool // a failure here degrades a feature rather than breaking forestry
}

func cmdDoctor(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("doctor takes no arguments")
	}
	var failed int
	for _, c := range checks {
		detail, err := c.run()
		switch {
		case err == nil:
			fmt.Printf("%s %-15s %s\n", okStyle.Render("✓"), c.name, dimStyle.Render(detail))
		case c.warn:
			fmt.Printf("%s %-15s %s\n", dirtyStyle.Render("!"), c.name, dirtyStyle.Render(err.Error()))
		default:
			failed++
			fmt.Printf("%s %-15s %s\n", errStyle.Render("✗"), c.name, errStyle.Render(err.Error()))
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

func checkGit() (string, error) {
	out, err := git("--version")
	if err != nil {
		return "", errors.New("git not found on PATH — forestry is a wrapper around it")
	}
	return out, nil
}

func checkRepo() (string, error) {
	repo, err := mainWorktree()
	if err != nil {
		return "", err
	}
	return tilde(repo), nil
}

func checkWorktrees() (string, error) {
	list, err := worktrees()
	if err != nil {
		return "", err
	}
	var missing []string
	for _, wt := range list {
		if _, err := os.Stat(wt.Path); err != nil {
			missing = append(missing, wt.Name())
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("%s listed but not on disk — try `git worktree prune`", strings.Join(missing, ", "))
	}
	return fmt.Sprintf("%d listed, all present", len(list)), nil
}

// checkRoot verifies new worktrees can actually be written where they go.
func checkRoot() (string, error) {
	repo, err := mainWorktree()
	if err != nil {
		return "", err
	}
	root := worktreeRoot(repo)
	dir := existingAncestor(root)
	if dir == "" {
		return "", fmt.Errorf("%s: no existing parent directory", tilde(root))
	}
	tmp, err := os.MkdirTemp(dir, ".forestry-doctor-")
	if err != nil {
		return "", fmt.Errorf("%s is not writable: %v", tilde(dir), err)
	}
	os.Remove(tmp)
	return tilde(root), nil
}

// existingAncestor is path, or the closest of its parents that exists.
func existingAncestor(path string) string {
	for {
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func checkConfig() (string, error) {
	path := configPath()
	if path == "" {
		return "", errors.New("no home directory — config file cannot be found")
	}
	if _, err := os.Stat(path); err != nil {
		return tilde(path) + " (none, using defaults)", nil
	}
	var set []string
	for _, key := range []string{"root", "editor"} {
		if v := configValue(key); v != "" {
			set = append(set, key+"="+v)
		}
	}
	if len(set) == 0 {
		return "", fmt.Errorf("%s has no root or editor key — is it `key = value`?", tilde(path))
	}
	return tilde(path) + ": " + strings.Join(set, ", "), nil
}

// checkGitHub runs the real pull request lookup the list column uses.
func checkGitHub() (string, error) {
	if _, err := command("gh", "--version"); err != nil {
		return "", errors.New("gh not on PATH — pull request column falls back to local merge detection")
	}
	prs, err := ghPullRequests()
	if err != nil {
		return "", fmt.Errorf("gh pr list failed: %s — try `gh auth login`", firstLine(err.Error()))
	}
	return fmt.Sprintf("gh ok, %d pull request(s) for this repo", len(prs)), nil
}

// firstLine keeps a chatty tool's error to one line of output.
func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

func checkFallback() (string, error) {
	base := defaultBranch()
	if base == "" {
		return "", errors.New("no default branch — merged branches go unnoticed without gh")
	}
	return base, nil
}

func checkShell() (string, error) {
	sh := os.Getenv("SHELL")
	if sh == "" {
		return "", errors.New("$SHELL unset — enter opens /bin/sh")
	}
	if _, err := os.Stat(sh); err != nil {
		return "", fmt.Errorf("$SHELL is %s, which does not exist", sh)
	}
	return sh, nil
}

func checkEditor() (string, error) {
	argv := editorCommand()
	if len(argv) == 0 {
		return "", errors.New(`no editor set — e does nothing; try FORESTRY_EDITOR="code -n" (or VISUAL, EDITOR)`)
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return "", fmt.Errorf("editor %q not found on PATH", argv[0])
	}
	return strings.Join(argv, " "), nil
}
