package main

import (
	"fmt"
	"os"
)

const usage = `forestry — manage git worktrees

Usage:
  forestry                            start interactive mode
  forestry list                       list the worktrees of this repo
  forestry new <name> [--from <ref>]  create a worktree on branch <name>
  forestry remove <name> [--force]    remove a worktree
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
	case "remove", "rm":
		return cmdRemove(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	return fmt.Errorf("unknown command %q — try 'help'", cmd)
}
