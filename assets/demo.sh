#!/bin/sh
# Make the demo GIF for the README.
# Usage: sh assets/demo.sh
# Needs: asciinema, agg (brew install asciinema agg) and expect.
set -e
root=$(cd "$(dirname "$0")/.." && pwd)
demo=$(cd "${TMPDIR:-/tmp}" && pwd -P)/forestry-demo
rm -rf "$demo"
mkdir -p "$demo/bin"
go build -o "$demo/bin/forestry" "$root/cmd/forestry"
export PATH="$demo/bin:$PATH"

# A small repository with two worktrees, one of them dirty.
export HOME="$demo"
git init -q -b main "$demo/myrepo"
cd "$demo/myrepo"
git -c user.name=demo -c user.email=demo@example.com commit -q --allow-empty -m "init"
forestry new bugfix-123 >/dev/null
forestry new feat-login >/dev/null
echo change > ../myrepo-worktrees/feat-login/login.go

export TERM=xterm-256color COLORTERM=truecolor
asciinema rec --headless --overwrite --window-size 128x14 \
  -c "expect $root/assets/demo.exp" "$demo/demo.cast"
agg --font-size 14 --theme monokai "$demo/demo.cast" "$root/assets/demo.gif"
