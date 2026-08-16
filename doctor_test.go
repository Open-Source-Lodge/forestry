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
