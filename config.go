package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// worktreeRoot is the directory holding the worktrees of the repo checked out
// at repoPath. It is, in order of precedence:
//
//	$FORESTRY_ROOT/<repo>
//	<root from ~/.forestry>/<repo>
//	<repo parent>/<repo>-worktrees
func worktreeRoot(repoPath string) string {
	repo := filepath.Base(repoPath)
	if root := os.Getenv("FORESTRY_ROOT"); root != "" {
		return filepath.Join(expandHome(root), repo)
	}
	if root := configValue("root"); root != "" {
		return filepath.Join(expandHome(root), repo)
	}
	return filepath.Join(filepath.Dir(repoPath), repo+"-worktrees")
}

// configPath is your own settings file: same name and format as a repo's
// checked-in `.forestry`, for the settings you want in every repo.
func configPath() string { return expandHome("~/.forestry") }

// editorCommand is the editor to open a worktree with, as command and
// arguments. It is, in order of precedence:
//
//	$FORESTRY_EDITOR
//	<editor from ~/.forestry>
//	$VISUAL
//	$EDITOR
func editorCommand() []string {
	for _, v := range []string{
		os.Getenv("FORESTRY_EDITOR"),
		configValue("editor"),
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
	} {
		if fields := strings.Fields(v); len(fields) > 0 {
			return fields
		}
	}
	return nil
}

// repoValue reads key from the repo's `.forestry` file, which is meant to be
// checked in so a repo's settings travel with it, and falls back to your own
// `~/.forestry`. Only for settings a repo may decide for everyone working in it:
// a setting that names a path or a command goes through configValue instead, so
// that cloning a repo cannot pick what runs on your machine.
func repoValue(key string) string {
	if path, err := repoConfigPath(); err == nil {
		if v := fileValue(path, key); v != "" {
			return v
		}
	}
	return configValue(key)
}

// repoConfigPath is the `.forestry` of the worktree the command runs in — the
// checkout you are working in decides, not whatever the main worktree has out.
func repoConfigPath() (string, error) {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".forestry"), nil
}

// deleteBranchOnRemove reports whether removing a worktree should also delete
// the local branch it had checked out.
func deleteBranchOnRemove() bool {
	on, _ := strconv.ParseBool(repoValue("delete_branch"))
	return on
}

// configValue reads key from your own `~/.forestry`.
func configValue(key string) string { return fileValue(configPath(), key) }

// fileValue reads key from a config file. The format is a minimal subset of
// TOML: `key = value` lines with `#` comments.
func fileValue(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		k, val, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(val), `"'`)
	}
	return ""
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}
