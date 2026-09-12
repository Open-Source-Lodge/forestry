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
// degrades (forestry still works, just with less), and set needs to the check
// this one builds on, so one broken thing reports one failure.
var checks = []check{
	{name: "git", run: checkGit},
	{name: "repository", run: checkRepo, needs: "git"},
	{name: "worktrees", run: checkWorktrees, needs: "repository"},
	{name: "worktree root", run: checkRoot, needs: "repository"},
	{name: "config", run: checkConfig, warn: true},
	{name: "github", run: checkGitHub, warn: true, needs: "repository"},
	{name: "merge fallback", run: checkFallback, warn: true, needs: "repository"},
	{name: "pull refs", run: checkPullRefs, warn: true, needs: "repository"},
	{name: "shell", run: checkShell, warn: true},
	{name: "open shells", run: checkOpenShells, warn: true},
	{name: "editor", run: checkEditor, warn: true},
}

// check reports on one thing forestry needs. The detail is shown either way.
type check struct {
	name string
	run  func() (string, error)
	warn bool // a failure here degrades a feature rather than breaking forestry
	// needs names the check this one builds on. When that one failed, this is
	// skipped rather than run to repeat the same cause in another tool's words.
	needs string
}

func cmdDoctor(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("doctor takes no arguments")
	}
	var failed int
	broken := map[string]bool{}
	for _, c := range checks {
		if broken[c.needs] {
			// Skipping proves nothing, so anything built on this one goes too.
			broken[c.name] = true
			fmt.Printf("%s %-15s %s\n", dimStyle.Render("-"), c.name, dimStyle.Render("skipped — needs "+c.needs))
			continue
		}
		detail, err := c.run()
		if err != nil {
			broken[c.name] = true
		}
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
			// Only a gone directory means prune; anything else is its own problem.
			if !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("%s is unreadable: %v", wt.Name(), err)
			}
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
	// The `.forestry` files in play, nearest first.
	paths := []string{configPath()}
	if path, err := repoConfigPath(); err == nil {
		paths = append([]string{path}, paths...)
	}
	var found []string
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("%s is unreadable: %v — forestry ignores it in silence", tilde(path), err)
			}
			continue
		}
		var set []string
		for _, key := range []string{"root", "editor", "delete_branch"} {
			if v := fileValue(path, key); v != "" {
				set = append(set, key+"="+v)
			}
		}
		if len(set) == 0 {
			return "", fmt.Errorf("%s sets nothing forestry knows — is it `key = value`?", tilde(path))
		}
		found = append(found, tilde(path)+": "+strings.Join(set, ", "))
	}
	if len(found) == 0 {
		return "no .forestry file (using defaults)", nil
	}
	return strings.Join(found, "; "), nil
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

// checkPullRefs verifies the remote `forestry pr` fetches pull request branches from.
func checkPullRefs() (string, error) {
	url, err := git("remote", "get-url", "origin")
	if err != nil {
		return "", errors.New("no remote named origin — `forestry pr` cannot fetch pull request branches")
	}
	return url, nil
}

func checkShell() (string, error) {
	sh := os.Getenv("SHELL")
	if sh == "" {
		return "", errors.New("$SHELL unset — enter opens /bin/sh")
	}
	if _, err := os.Stat(sh); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("$SHELL is %s, which is unreadable: %v", sh, err)
		}
		return "", fmt.Errorf("$SHELL is %s, which does not exist", sh)
	}
	return sh, nil
}

// checkOpenShells verifies the shell registry is writable, and counts the
// shells that are open now.
func checkOpenShells() (string, error) {
	f, err := os.OpenFile(shellsPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("%s is not writable: %v. Forestry cannot record open shells", tilde(shellsPath()), err)
	}
	f.Close()
	n := 0
	for _, list := range openShells() {
		n += len(list)
	}
	return fmt.Sprintf("%s, %d open", tilde(shellsPath()), n), nil
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
