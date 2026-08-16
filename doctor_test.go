package main

import (
	"path/filepath"
	"testing"
)

func TestExistingAncestor(t *testing.T) {
	dir := t.TempDir()
	if got := existingAncestor(dir); got != dir {
		t.Errorf("existing dir: got %q, want %q", got, dir)
	}
	deep := filepath.Join(dir, "a", "b", "c")
	if got := existingAncestor(deep); got != dir {
		t.Errorf("missing dir: got %q, want %q", got, dir)
	}
	if got := existingAncestor("/nope/nowhere"); got != "/" {
		t.Errorf("absent path: got %q, want %q", got, "/")
	}
}

// A check can only depend on one listed above it, or the skip never triggers.
func TestChecksDependOnEarlierChecks(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range checks {
		if c.needs != "" && !seen[c.needs] {
			t.Errorf("check %q needs %q, which is not defined before it", c.name, c.needs)
		}
		seen[c.name] = true
	}
}

// Every check must say something, whether it passes or fails.
func TestChecksSayWhy(t *testing.T) {
	for _, c := range checks {
		detail, err := c.run()
		if err == nil && detail == "" {
			t.Errorf("check %q passed with no detail", c.name)
		}
		if err != nil && err.Error() == "" {
			t.Errorf("check %q failed with no reason", c.name)
		}
	}
}
