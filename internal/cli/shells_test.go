package cli

import (
	"os"
	"strings"
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

func TestOpenShellTabInTmux(t *testing.T) {
	var got []string
	stubCommand(t, func(name string, args ...string) (string, error) {
		got = append([]string{name}, args...)
		return "", nil
	})
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")

	if err := openShellTab("/wt/a b"); err != nil {
		t.Fatal(err)
	}
	if got[0] != "tmux" || got[1] != "new-window" || got[3] != "/wt/a b" {
		t.Errorf("tmux call = %v", got)
	}
	if !strings.Contains(got[4], "shell '/wt/a b'") {
		t.Errorf("window command = %q, want it to run forestry shell", got[4])
	}
}

func TestOpenShellTabWithoutTerminal(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TERM_PROGRAM", "")
	if err := openShellTab("/wt/a"); err == nil {
		t.Error("openShellTab must fail without tmux or a known terminal")
	}
}

func TestShq(t *testing.T) {
	if got := shq(`a'b`); got != `'a'\''b'` {
		t.Errorf("shq = %q", got)
	}
	if got := asq(`a"b\c`); got != `a\"b\\c` {
		t.Errorf("asq = %q", got)
	}
}
