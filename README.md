# forestry

A small terminal UI and CLI for managing git worktrees. It shells out to plain
`git` commands — there is no state of its own, so anything you do with
`git worktree` directly stays visible to forestry, and vice versa.

## Install

```sh
make build                                              # then move bin/forestry onto your PATH
go install github.com/Open-Source-Lodge/forestry@latest
```

## Usage

```
forestry                            start interactive mode
forestry list                       list the worktrees of this repo
forestry new <name> [--from <ref>]  create a worktree on branch <name>
forestry remove <name> [--force]    remove a worktree
forestry doctor                     check that everything forestry needs works
forestry help                       show help
```

`*` in `list` output marks the worktree you are currently in.

## Interactive mode

Run `forestry` with no arguments to browse the worktrees and act on the
selected one:

```
  forestry · myrepo

❯ • myrepo      main        main     ~/github/myrepo
    bugfix-123  bugfix-123  clean    merged #7  ~/github/myrepo-worktrees/bugfix-123
    feat-login  feat/login  dirty    open #12   ~/github/myrepo-worktrees/feat-login

  ↑↓ move · enter shell · e editor · n new · d remove · r refresh · q quit
```

| key           | action                                                |
| ------------- | ----------------------------------------------------- |
| `↑` `↓` `k` `j` | move the selection                                  |
| `g` `G`       | jump to the first / last worktree                     |
| `enter`       | open a shell in the selected worktree                 |
| `e`           | open the selected worktree in your editor             |
| `n`           | create a worktree (branch name, optional base ref)    |
| `d`           | remove the selected worktree, with a confirmation     |
| `r`           | reload the list                                       |
| `q` `esc`     | quit                                                  |

`•` marks the worktree you are currently in, and the selection starts there.

`enter` runs `$SHELL` with its working directory set to the worktree and
`FORESTRY_WORKTREE` pointing at it; leave the shell and you are back at the
list. A program cannot change the directory of the shell that started it, so
this is how you get *into* a worktree rather than just at it.

`e` runs your editor with the worktree directory as its argument. The editor is
`$FORESTRY_EDITOR`, then `editor` from the config file, then `$VISUAL`, then
`$EDITOR` — the first one set wins, and each may carry arguments:

```sh
export FORESTRY_EDITOR="code -n"   # or "zed", "nvim", "subl -n", "idea"
```

```toml
editor = "code -n"
```

The value is the command to run, not a path to a file: forestry splits it on
spaces and appends the worktree directory, so `code -n` runs `code -n <path>`.

A terminal editor keeps the screen until you leave it; a windowed one like VS
Code opens and drops you straight back at the list.

`d` asks before removing. Git refuses to remove a worktree with uncommitted
changes, so the confirmation offers `f` to force it.

### Pull requests

The column after the status shows the pull request of the worktree's branch —
`merged`, `open`, `draft` or `closed`, with its number. `merged` is the
interesting one: the work has landed, so the worktree is a candidate for `d`.

This comes from the [GitHub CLI](https://cli.github.com), and needs `gh` on your
PATH and logged in. Since that means a network call, the list appears first and
the column fills in a moment later.

Without `gh`, forestry falls back to asking git whether the branch is contained
in the default branch, and shows `merged` when it is. That needs no network but
only sees real merges — a squashed or rebased pull request leaves no trace of
the branch in history, so it goes unnoticed. Repositories with no remote at all
get an empty column.

### new

`forestry new <name>` creates branch `<name>` from the current `HEAD` and checks
it out in a new worktree. Use `--from <ref>` to branch from somewhere else:

```sh
forestry new feat/login              # branch from HEAD
forestry new hotfix --from v1.2.0    # branch from a tag
forestry new hotfix --from origin/main
forestry new existing-branch         # no --from and branch exists → check it out
```

Branch names may contain slashes; the directory name flattens them, so
`feat/login` lives in a directory called `feat-login`.

### remove

`forestry remove <name>` takes the directory name shown by `list`. Git refuses to
remove a worktree with uncommitted changes unless you pass `--force`. The branch
itself is left alone — delete it with `git branch -d <name>` if you want it gone.

The main worktree can never be removed.

### doctor

`forestry doctor` checks everything forestry depends on and says what is wrong
when something is:

```
✓ git             git version 2.50.1
✓ repository      ~/github/myrepo
✓ worktrees       3 listed, all present
✓ worktree root   ~/github/myrepo-worktrees
✓ config          ~/.config/forestry/config.toml: root=~/worktrees
! github          gh not on PATH — pull request column falls back to local merge detection
✓ merge fallback  origin/main
✓ shell           /bin/zsh
! editor          no editor set — e does nothing; set FORESTRY_EDITOR, VISUAL or EDITOR
```

`✗` is a failure: forestry cannot work until it is fixed, and the exit status is
non-zero. `!` is a warning: that one feature is degraded, everything else works,
and the exit status stays zero. The github check runs the real `gh pr list`
call the pull request column uses, so it catches an expired login too.

Checks build on each other: no `git` means no repository, and no repository
means nothing to ask about worktrees or pull requests. A check whose
prerequisite failed is skipped with `-` rather than run, so one broken thing
reports one failure instead of restating itself in each tool's own words.

## Contributing

The `Makefile` wraps the usual commands:

| command        | what it does                                |
| -------------- | ------------------------------------------- |
| `make`         | `vet`, `test` and `build`                   |
| `make build`   | build `bin/forestry`                        |
| `make run`     | `go run .`                                  |
| `make test`    | `go test ./...`                             |
| `make cover`   | tests with coverage, as CI runs them        |
| `make vet`     | `go vet ./...`                              |
| `make fmt`     | `go fmt ./...`                              |
| `make install` | install into `$GOBIN`                       |
| `make clean`   | remove `bin/`                               |

**Every new feature gets a doctor check.** If a feature depends on anything
outside the process — a binary on `PATH`, an environment variable, a directory
forestry writes to, a network call — add an entry to the `checks` table in
`doctor.go` and a line to the sample output above. Fail the check when forestry
breaks without it, warn when only that feature degrades, and set `needs` to the
check it builds on so a shared cause is reported once. That way `forestry
doctor` stays an honest answer to "does everything work?" instead of drifting
into a list of the things that mattered in 2025.

## Where worktrees go

By default, next to the repository, in `<repo>-worktrees/`:

```
~/github/
├── myrepo/                  # main checkout
└── myrepo-worktrees/
    ├── feat-login/
    └── bugfix-123/
```

This keeps worktrees out of the repository itself, so they never show up in
`git status` or get picked up by editors and build tools.

To collect every repo's worktrees under one directory instead, set a root in
`~/.config/forestry/config.toml` (or `$XDG_CONFIG_HOME/forestry/config.toml`):

```toml
root = "~/worktrees"
```

Worktrees are then namespaced per repository, so different repos can use the
same branch name without colliding:

```
~/worktrees/myrepo/feat-login
~/worktrees/otherrepo/feat-login
```

`FORESTRY_ROOT` overrides the config file, which is handy for one-off runs:

```sh
FORESTRY_ROOT=/tmp/scratch forestry new experiment
```
