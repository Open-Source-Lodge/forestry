package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

func cmdList(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("list takes no arguments")
	}
	list, err := worktrees()
	if err != nil {
		return err
	}
	current, _ := git("rev-parse", "--show-toplevel")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tBRANCH\tSTATUS\tPATH")
	for _, wt := range list {
		marker := " "
		if wt.Path == current {
			marker = "*"
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n", marker, wt.Name(), wt.Ref(), status(wt), wt.Path)
	}
	return w.Flush()
}

func status(wt Worktree) string {
	var parts []string
	if wt.Main {
		parts = append(parts, "main")
	}
	if isDirty(wt.Path) {
		parts = append(parts, "dirty")
	}
	if wt.Locked {
		parts = append(parts, "locked")
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, ",")
}

func cmdNew(args []string) error {
	var name, from string
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--from":
			if i+1 >= len(args) {
				return errors.New("--from needs a ref")
			}
			i++
			from = args[i]
		case strings.HasPrefix(arg, "--from="):
			from = strings.TrimPrefix(arg, "--from=")
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown flag %q", arg)
		case name == "":
			name = arg
		default:
			return fmt.Errorf("unexpected argument %q", arg)
		}
	}
	path, err := createWorktree(name, from)
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

func cmdPR(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: forestry pr <number>")
	}
	path, err := createFromPR(args[0])
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

// createWorktree checks branch name out in a fresh worktree and returns its path.
func createWorktree(name, from string) (string, error) {
	if err := validName(name); err != nil {
		return "", err
	}
	repo, err := mainWorktree()
	if err != nil {
		return "", err
	}
	// Branch names may contain slashes; the directory name must not nest.
	dir := strings.ReplaceAll(name, "/", "-")
	path := filepath.Join(worktreeRoot(repo), dir)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := addWorktree(path, name, from); err != nil {
		os.Remove(filepath.Dir(path))
		return "", err
	}
	return path, nil
}

// addWorktree reuses branch name if it already exists and no base was asked for.
func addWorktree(path, name, from string) error {
	if from == "" && branchExists(name) {
		_, err := git("worktree", "add", path, name)
		return err
	}
	base := from
	if base == "" {
		base = "HEAD"
	}
	_, err := git("worktree", "add", "-b", name, path, base)
	return err
}

func validName(name string) error {
	switch {
	case name == "":
		return errors.New("usage: forestry new <name> [--from <ref>]")
	case filepath.IsAbs(name), strings.HasPrefix(name, "."):
		return fmt.Errorf("invalid name %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid name %q", name)
		}
	}
	return nil
}

func cmdRemove(args []string) error {
	var name string
	var force bool
	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag %q", arg)
			}
			if name != "" {
				return fmt.Errorf("unexpected argument %q", arg)
			}
			name = arg
		}
	}
	if name == "" {
		return errors.New("usage: forestry remove <name> [--force]")
	}

	wt, err := findWorktree(name)
	if err != nil {
		return err
	}
	here := insideDir(wt.Path)
	if err := removeWorktree(wt, force); err != nil {
		return err
	}
	fmt.Printf("removed %s\n", wt.Path)
	if here {
		// We cannot move the shell that ran us; tell it where to go.
		if repo, err := os.Getwd(); err == nil {
			fmt.Printf("your shell is in a deleted directory — cd %s\n", repo)
		}
	}
	return nil
}

func removeWorktree(wt Worktree, force bool) error {
	if wt.Main {
		return errors.New("refusing to remove the main worktree")
	}
	// Step out first: every git call after this one fails if our working
	// directory is the tree that just got deleted.
	if insideDir(wt.Path) {
		repo, err := mainWorktree()
		if err != nil {
			return err
		}
		if err := os.Chdir(repo); err != nil {
			return err
		}
	}
	rm := []string{"worktree", "remove", wt.Path}
	if force {
		rm = append(rm, "--force")
	}
	_, err := git(rm...)
	return err
}

// insideDir reports whether the working directory is dir or below it.
func insideDir(dir string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		// The directory is already gone from under us; stepping out is still right.
		return true
	}
	// Symlinked paths (/tmp on macOS) must compare against what git reports.
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	rel, err := filepath.Rel(dir, cwd)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
