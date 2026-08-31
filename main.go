package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

const usage = `forestry — manage git worktrees

Usage:
  forestry                            start interactive mode
  forestry list                       list the worktrees of this repo
  forestry new <name> [--from <ref>]  create a worktree on branch <name>
  forestry pr <number>                create a worktree from a pull request
  forestry remove <name> [--force]    remove a worktree
  forestry doctor                     check that everything forestry needs works
  forestry version                    show the forestry version
  forestry help                       show this help

New branches are created from HEAD unless --from is given.
Worktrees live in ../<repo>-worktrees/ by default; see README for config.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "forestry: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return tui()
	}
	return dispatch(args[0], args[1:])
}

func dispatch(cmd string, args []string) error {
	switch cmd {
	case "list", "ls":
		return cmdList(args)
	case "new", "add":
		return cmdNew(args)
	case "pr":
		return cmdPR(args)
	case "remove", "rm":
		return cmdRemove(args)
	case "doctor":
		return cmdDoctor(args)
	case "shell":
		// The `t` key in interactive mode runs this in the new tab, so that
		// the shell registers itself. It is not in the usage text.
		if len(args) != 1 {
			return fmt.Errorf("usage: forestry shell <path>")
		}
		return runShell(args[0])
	case "version", "-v", "--version":
		fmt.Println(version())
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	return fmt.Errorf("unknown command %q — try 'help'", cmd)
}

// version reads the module version that Go recorded in the binary. Builds
// made with `go install <module>@<tag>` get the tag. Builds made from a
// source checkout get "(devel)" or the version that ldflags set.
func version() string {
	if v := versionOverride; v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}

// versionOverride is set with ldflags in the release workflow, because a
// binary that a workflow builds from a checkout has no module version.
var versionOverride string
