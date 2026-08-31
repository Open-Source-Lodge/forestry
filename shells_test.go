package main

import (
	"os"
	"testing"
)

func TestShellRegistry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stubCommand(t, func(name string, args ...string) (string, error) {
		return "ttys001", nil // ps -o tty=
	})

	// Our own pid is alive; a pid far above any real pid table is not.
	registerShell(os.Getpid(), "/wt/a")
	registerShell(999999999, "/wt/b")

	open := openShells()
	if len(open["/wt/a"]) != 1 || open["/wt/a"][0].tty != "ttys001" {
		t.Fatalf("openShells()[/wt/a] = %v, want one shell on ttys001", open["/wt/a"])
	}
	if len(open["/wt/b"]) != 0 {
		t.Errorf("openShells kept a dead shell: %v", open["/wt/b"])
	}

	// The prune must also rewrite the file.
	if got := readShells(); len(got) != 1 {
		t.Errorf("readShells after prune = %v, want 1 entry", got)
	}

	unregisterShell(os.Getpid())
	if got := openShells(); len(got) != 0 {
		t.Errorf("openShells after unregister = %v, want none", got)
	}
}
