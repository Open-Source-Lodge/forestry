# forestry

Forestry is a terminal user interface and a command line tool. It manages git
worktrees. Forestry sends commands to the `git` program and keeps no data of
its own. Thus forestry shows the worktrees that you make with `git worktree`,
and `git worktree` shows the worktrees that you make with forestry.

## Install

```sh
make build                                              # then move bin/forestry into a directory on your PATH
go install github.com/Open-Source-Lodge/forestry@latest
```

Each [release](https://github.com/Open-Source-Lodge/forestry/releases) has
binaries for macOS and Linux, on amd64 and arm64. Download the binary for your
system, check it against `checksums.txt`, and move it into a directory on your
PATH.

## Commands

```
forestry                            start interactive mode
forestry list                       list the worktrees of this repository
forestry new <name> [--from <ref>]  make a worktree on branch <name>
forestry pr <number>                make a worktree from a pull request
forestry remove <name> [--force]    remove a worktree
forestry doctor                     make sure that forestry can operate
forestry version                    show the forestry version
forestry help                       show the help
```

In the output of `list`, the `*` mark shows the worktree that you are in. The
status shows `shell` when a shell from forestry is open in the worktree.

## Interactive mode

Start `forestry` with no arguments to see the worktrees. Then do an operation
on the worktree that you select:

```
  forestry · myrepo

❯ • myrepo      main        main     ~/github/myrepo
    bugfix-123  bugfix-123  clean    merged #7  ~/github/myrepo-worktrees/bugfix-123
    feat-login  feat/login  dirty    open #12   ~/github/myrepo-worktrees/feat-login

  ↑↓ move · enter shell · t shell tab · s find shell · e editor · n new · P from PR · d remove · p open PR · r refresh · q quit
```

| key             | operation                                            |
| --------------- | ---------------------------------------------------- |
| `↑` `↓` `k` `j` | move the selection                                   |
| `g` `G`         | go to the first or the last worktree                 |
| `enter`         | open a shell in the selected worktree                |
| `t`             | open a shell in a new terminal tab                   |
| `s`             | move the focus to the open shell of the worktree     |
| `e`             | open the selected worktree in your editor            |
| `n`             | make a worktree                                      |
| `P`             | select an open pull request, or type its number      |
| `p`             | open the pull request of the worktree in a browser   |
| `d`             | remove the selected worktree, after a confirmation   |

| `r`             | read the list again                                  |
| `q` `esc`       | stop forestry                                        |

In the confirmation, press `y` to remove the worktree. Press `Y` to remove
the worktree and also delete its local branch. Press `f` to remove a worktree
that has changes that you did not commit.

The `•` mark shows the worktree that you are in. The selection starts at that
worktree.

The `enter` key closes the interface. Then forestry starts `$SHELL` in the
directory of the worktree. The `FORESTRY_WORKTREE` variable contains the path
to that worktree.

Forestry records each shell that `enter` opens in `~/.forestry-shells`. The
status of a worktree with an open shell shows `shell`. When you close the
shell, forestry removes the record. Thus the status helps you find the
terminals that you already have open.

The `t` key opens a shell in a new terminal tab, and the forestry interface
stays open. In tmux, forestry opens a new tmux window. On macOS, forestry
opens a new tab in iTerm2, or a new window in Terminal. In a different
terminal program, the `t` key does not work. The new shell also registers
in `~/.forestry-shells`. A terminal cannot send the `ctrl+enter` or
`cmd+enter` keys to forestry, thus the key is `t`.

The `s` key moves the focus to the terminal of that shell. When the worktree
has more than one open shell, forestry shows a list of the shells. Select a
shell and push `enter` to move the focus to it. In tmux, forestry
switches the client to the pane of the shell. On macOS, forestry selects the
tab of Terminal or iTerm2 that has the shell. On a different system, or in a
different terminal program, forestry shows the terminal device of the shell. Then
you can find the terminal yourself.

The `e` key starts your editor. Forestry gives the directory of the worktree
to the editor as an argument. Forestry looks for the editor in this sequence:
`$FORESTRY_EDITOR`, then `editor` in the configuration file, then `$VISUAL`,
then `$EDITOR`. Forestry uses the first one that has a value. Each one can
include arguments:

```sh
export FORESTRY_EDITOR="code -n"   # or "zed", "nvim", "subl -n", "idea"
```

```toml
editor = "code -n"
```

The value is a command, not a path to a file. Forestry divides the value at
the spaces. Then forestry adds the directory of the worktree. Thus `code -n`
becomes the command `code -n <path>`.

A terminal editor keeps the screen until you close the editor. An editor with
its own window opens, and forestry shows the list again immediately.

The `d` key asks you before it removes the worktree. Git does not remove a
worktree that has changes that you did not commit. Push `f` at the
confirmation to force the removal.

### Pull requests

The column after the status shows the pull request for the branch of the
worktree. The status of the pull request is `merged`, `open`, `draft` or
`closed`, with the number of the pull request. When the status is `merged`,
the work is in the default branch. Thus you can remove that worktree with the
`d` key.

This data comes from the [GitHub CLI](https://cli.github.com). Install `gh` in
a directory on your PATH, and log in. The `gh` program makes a network
connection. Thus forestry shows the list first, and adds the column a moment
later.

Without `gh`, forestry asks git if the default branch contains the branch. If
the default branch contains the branch, forestry shows `merged`. This
alternative method makes no network connection, but it finds only true merges.
A squash merge or a rebase merge removes all record of the branch from the
history. Thus forestry does not find these merges. For a repository with no
remote, the column stays empty.

### new

The `forestry new <name>` command makes the branch `<name>` from the current
`HEAD`. Then it checks out that branch in a new worktree. Use `--from <ref>`
to make the branch from a different location:

```sh
forestry new feat/login              # make the branch from HEAD
forestry new hotfix --from v1.2.0    # make the branch from a tag
forestry new hotfix --from origin/main
forestry new existing-branch         # no --from, and the branch exists: check it out
```

If the branch exists on `origin` but not in your clone, forestry gets it from
`origin` and makes the worktree from that state. If the branch exists,
forestry ignores `--from` and checks the branch out as it is.

A branch name can contain slashes. In the name of the directory, forestry
replaces each slash with a hyphen. Thus the branch `feat/login` uses the
directory `feat-login`.

### pr

The `forestry pr <number>` command checks out the head branch of a pull
request in a new worktree:

```sh
forestry pr 42
forestry pr '#42'
```

The name of the branch comes from `gh`. If the branch is not local, forestry
gets the branch from `pull/<number>/head`. Thus a pull request from a fork
operates in the same manner as a pull request from a branch in the repository.
If a local branch with that name exists, forestry checks out that local branch
and gets no data from the remote.

### The list of pull requests

The `P` key in interactive mode shows the open pull requests. The most
recently changed pull request is first. Each page shows eight pull requests:

```
  Worktree from pull request

  #42   2026-08-14  Add worktrees from pull requests
❯ #41   2026-08-12  Bump bubbletea
  #7    2026-07-30  Fix doctor skip logic

  page 1/3 · 21 open

  number

  ↑↓ pick · ←→ page · type a number · enter create · esc cancel
```

The `↑↓` keys move the selection. The `←→` keys, or the `pgup` and `pgdn`
keys, move one page. The `enter` key checks out the selected pull request. A
number that you type has precedence over the selection. Thus you can also use
a pull request that the list does not show. Examples are a closed pull
request, or a pull request after the first 200 that `gh` supplies. Without
`gh`, the list is empty and gives the reason. A number that you type continues
to operate.

### remove

The `forestry remove <name>` command uses the name of the directory that
`list` shows. Git does not remove a worktree that has changes that you did not
commit. Use the `--force` flag to remove such a worktree. Forestry does not
remove the branch. To remove the branch, use the `git branch -d <name>`
command.

Forestry cannot remove the main worktree.

### doctor

The `forestry doctor` command examines all the tools and the data that
forestry uses. The command tells you which item has a fault:

```
✓ git             git version 2.50.1
✓ repository      ~/github/myrepo
✓ worktrees       3 listed, all present
✓ worktree root   ~/github/myrepo-worktrees
✓ config          ~/github/myrepo/.forestry: delete_branch=true; ~/.forestry: root=~/worktrees
! github          gh not on PATH — pull request column falls back to local merge detection
✓ merge fallback  origin/main
✓ pull refs       git@github.com:me/myrepo.git
✓ shell           /bin/zsh
✓ open shells     ~/.forestry-shells, 1 open
! editor          no editor set — e does nothing; set FORESTRY_EDITOR, VISUAL or EDITOR
```

The `✗` mark is a failure. Forestry cannot operate until you correct the
failure, and the exit status is not zero. The `!` mark is a warning. Only one
function is not fully available, all the other functions operate, and the exit
status is zero. The github check makes the same `gh pr list` call as the pull
request column. Thus the check also finds a login that is no longer valid.

A check can have a prerequisite check. Without `git`, forestry cannot find the
repository. Without the repository, forestry cannot ask about worktrees or
pull requests. If the prerequisite check fails, forestry does not do the
check. Forestry shows the `-` mark for that check. Thus one fault causes one
failure message, and not a message from each tool.

## Contribute

The `Makefile` contains these commands:

| command        | operation                                   |
| -------------- | ------------------------------------------- |
| `make`         | `vet`, `test` and `build`                   |
| `make build`   | build `bin/forestry`                        |
| `make run`     | `go run .`                                  |
| `make test`    | `go test ./...`                             |
| `make cover`   | test with coverage, as CI does              |
| `make vet`     | `go vet ./...`                              |
| `make fmt`     | `go fmt ./...`                              |
| `make install` | install into `$GOBIN`                       |
| `make clean`   | remove `bin/`                               |

**Add a doctor check for each new function.** A function can have a
dependency outside of the process. Examples are a program on the `PATH`, an
environment variable, a directory that forestry writes to, or a network
connection. For each such dependency, add an entry to the `checks` table in
`doctor.go`. Also add a line to the example output above. Make the check fail
when forestry cannot operate without the dependency. Make the check give a
warning when only one function is not fully available. Set `needs` to the
prerequisite check. Then forestry reports a shared cause one time. Thus
`forestry doctor` continues to tell you if all the functions operate.

### Release

Read [`DEVELOPMENT.md`](DEVELOPMENT.md) for the release procedure.

### Documentation rules

Write all documentation in this repository in ASD-STE100 Simplified Technical
English. These are the primary rules:

- Use the words from the STE dictionary. Technical names, such as `worktree`,
  `branch` and `commit`, are permitted.
- Give one meaning to each word. Do not use a word as a noun and as a verb.
- Use the same word for the same thing in all the documents.
- Write short sentences. Use a maximum of 20 words in an instruction, and a
  maximum of 25 words in a description.
- Write one instruction in one sentence.
- Use the active voice. Do not use the passive voice.
- Use the simple present tense, the simple past tense or the simple future
  tense.
- Do not use the `-ing` form of a verb, unless it is part of a technical name.
- Use the articles `the`, `a` and `an` where possible.
- Do not remove words to make a sentence shorter.
- Write a maximum of six sentences in a paragraph.
- Write about one topic in one paragraph.
- Do not use contractions, idioms, slang or jargon.
- Do not use words from a different language.

Text in a code block shows the output of the program. Do not change that text
in the documentation. Change the program first.

## Where forestry puts the worktrees

The default location is adjacent to the repository, in `<repo>-worktrees/`:

```
~/github/
├── myrepo/                  # main checkout
└── myrepo-worktrees/
    ├── feat-login/
    └── bugfix-123/
```

This location keeps the worktrees out of the repository. Thus `git status`
does not show them, and editors and build tools do not read them.

To collect every repo's worktrees under one directory instead, set a root in
`~/.forestry`:

```toml
root = "~/worktrees"
```

Forestry then makes a directory for each repository. Thus two repositories can
use the same branch name:

```
~/worktrees/myrepo/feat-login
~/worktrees/otherrepo/feat-login
```

`FORESTRY_ROOT` overrides the settings file, which is handy for one-off runs:

```sh
FORESTRY_ROOT=/tmp/scratch forestry new experiment
```

## Settings

Settings live in `.forestry` files — `key = value` lines with `#` comments:

```toml
root = "~/worktrees"                 # where worktrees go
editor = "code -n"                   # what e opens
delete_branch = true                 # delete the local branch with its worktree
```

`~/.forestry` is yours and applies everywhere. A repository can also carry its
own `.forestry` at its root, checked into git so everyone working on it gets the
same behaviour, and it wins over `~/.forestry` where the two disagree.

Only `delete_branch` is read from a repository's file. `root` and `editor` name
a path and a command, and are read from `~/.forestry` alone — cloning a
repository must never decide what runs on your machine or where forestry writes.

| setting         | from                        | default                     |
| --------------- | --------------------------- | --------------------------- |
| `root`          | `~/.forestry`               | `<repo>-worktrees` next to the repo |
| `editor`        | `~/.forestry`               | `$VISUAL`, then `$EDITOR`   |
| `delete_branch` | repo `.forestry`, then `~/` | `false`                     |

With `delete_branch = true`, `forestry remove` also runs `git branch -d` on the
branch the worktree had checked out — `-D` when you pass `--force`. If the
branch has unmerged commits the worktree is still removed and the branch kept,
and forestry says so.
