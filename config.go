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
//	<root from config file>/<repo>
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

func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "forestry", "config.toml")
}

// editorCommand is the editor to open a worktree with, as command and
// arguments. It is, in order of precedence:
//
//	$FORESTRY_EDITOR
//	<editor from config file>
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

// forestryValue reads key from the repo's `.forestry` file, which is meant to be
// checked in so a repo's settings travel with it, and falls back to `~/.forestry`
// for the settings you want in every repo.
func forestryValue(key string) string {
	if repo, err := mainWorktree(); err == nil {
		if v := fileValue(filepath.Join(repo, ".forestry"), key); v != "" {
			return v
		}
	}
	return fileValue(expandHome("~/.forestry"), key)
}

// deleteBranchOnRemove reports whether removing a worktree should also delete
// the local branch it had checked out.
func deleteBranchOnRemove() bool {
	on, _ := strconv.ParseBool(forestryValue("delete_branch"))
	return on
}

// configValue reads key from the user's config file.
func configValue(key string) string {
	path := configPath()
	if path == "" {
		return ""
	}
	return fileValue(path, key)
}

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
