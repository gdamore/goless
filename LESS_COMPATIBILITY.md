# less Compatibility Audit

Issue: `#37`  
Repository scope: bundled `LessKeyGroup` plus the standalone program in `cmd/goless`

## Compatibility Target

The goal is not to reproduce GNU `less` exactly. The goal is to feel familiar to
most `less` users in the common paths:

- open a file or stream
- move around with expected keys
- search with expected prompts
- switch between files
- use a recognizable subset of CLI flags and startup directives

Security and embeddability still win when there is a conflict. Features that
require shell execution, raw terminal passthrough, or inheriting `less`'s more
dangerous integration points should stay out of scope.

## Current Snapshot

### Interactive Commands and Prompts

Already compatible or close enough:

- `/pattern`, `?pattern`, `n`, and `N`
- `:123` for line jump
- `:50%` for percentage jump
- `:q` and `:quit`
- `:n` and `:p` through the standalone program's command handler
- `:f` through the standalone program's `file` / `f` command alias
- `+42` and `+/pattern` startup directives

Useful, but intentionally different:

- `:set ...` commands are custom pager commands, not `less` option toggles
- search mode and case mode are exposed directly with `F2`, `F3`, and `:set`
- follow mode is exposed directly with `F`

Notable gaps:

- no `:Q`, `Q`, or `ZZ` quit aliases
- no `=` or `Ctrl-G` file-info/status command
- no `:x` alias for the first file
- no `:e [file]` command to examine a new file
- no support for marks, bracket matching, or tag navigation
- no file-spanning search commands such as `ESC-n` / `ESC-N`

### Keybindings

Compatible today:

- `q`, `Esc`
- `j` / `k` and arrow keys
- `space`, `f`, `b`
- `g`, `G`
- `/`, `?`, `n`, `N`
- `F` for follow mode

Close, but thinner than `less`:

- page navigation exists, but many familiar aliases are missing
- horizontal navigation exists, but not under the usual `less` bindings
- help exists on `H` / `F1`, not on lowercase `h`

Compatibility conflicts in the current bundled keymap:

- `<` and `>` are used for fine horizontal scrolling, but `less` uses them for
  top and bottom of file
- `w` toggles wrap, but `less` uses `w` for backward-window behavior
- `h` scrolls left, but `less` users often expect `h` / `H` to be help

High-value missing aliases:

- forward one line: `Enter`, `e`, `Ctrl-E`, `Ctrl-N`
- backward one line: `y`, `Ctrl-Y`, `Ctrl-K`, `Ctrl-P`
- forward one page: `Ctrl-F`, `Ctrl-V`
- backward one page: `Ctrl-B`, `Esc-v`
- top / bottom aliases: `<`, `>`

### CLI Switches and Startup Syntax

Compatible today:

- `-e` / `--quit-at-eof`
- `-E` / `--QUIT-AT-EOF`
- `+line`
- `+/pattern`
- multiple files plus explicit `-` for stdin

Current CLI is thinner than `less` in several common areas:

- no `-N` line-number alias
- no `-S` chop-long-lines alias
- no `-i` / `-I` search-case aliases
- no `-x N` tab-width alias
- no `-F` quit-if-one-screen behavior
- no standard `--help` / `-?` less-style help alias

Custom flags such as `-preset`, `-chrome`, `-hidden`, `-render`, and
`-live-links` are fine; they do not block familiarity as long as common `less`
habits also work.

## Recommended Changes

These are the changes most likely to improve familiarity without dragging the
project into full GNU `less` emulation.

### 1. Expand the bundled less-like key aliases

This is the highest-value compatibility work.

Add the common motion aliases that do not require larger parser changes:

- `Enter`, `e`, `Ctrl-E`, `Ctrl-N` for forward one line
- `y`, `Ctrl-Y`, `Ctrl-K`, `Ctrl-P` for backward one line
- `Ctrl-F`, `Ctrl-V` for page down
- `Ctrl-B`, `Esc-v` for page up
- `Q` as an additional quit key

Change the bundled keymap so:

- `<` goes to top of file
- `>` goes to bottom of file

Keep fine horizontal scrolling on the existing shifted-arrow bindings. This is a
better trade than spending two prominent `less` keys on a niche horizontal
motion variant.

### 2. Add the most common standalone program command aliases

The command prompt should accept more of the muscle-memory forms users expect:

- `:Q`
- `:x`
- `=`
- `Ctrl-G`

`=` and `Ctrl-G` should report the same current-file/status information already
available through `:file`.

### 3. Add common CLI aliases with low behavioral risk

These flags map cleanly onto existing pager behavior:

- `-N` to enable line numbers
- `-i` to keep smart-case behavior explicit
- `-I` to force case-insensitive search
- `-x N` to set tab width
- `-F` to quit if the entire file fits on the first screen

`-S` can also be accepted as an alias for no-wrap mode even though that is
already the default. That keeps scripts and muscle memory working without
forcing users to know the current default.

### 4. Treat numeric prefixes as a separate, deliberate follow-up

Counts are a real compatibility gap, but they are not a one-line alias patch.
Doing them well means deciding how prefixes compose with:

- motion keys
- search repeat
- `g`, `G`, `%`, and file commands
- prompt entry and multi-key commands

This is worth doing, but it should be its own follow-up item rather than being
folded into small keymap cleanup.

## Selected Non-Goals

These should remain intentionally unsupported unless the project direction
changes:

- shell escape commands such as `!`, `#`, and `|`
- editor launch via `v`
- save-to-file commands
- `LESSOPEN`, lesskey, and other external-process integration points
- raw control-character passthrough modes such as `-r` / `-R`
- exact GNU `less` prompt formatting and option-toggling syntax

Those features either conflict with the security model or add substantial
surface area without materially improving the experience for most users.

## Suggested Follow-Up Issues

- Add common `less` motion aliases and reclaim `<` / `>` for top and bottom
- Add standalone program aliases for quit and file-info commands
- Add common `less` CLI aliases (`-N`, `-S`, `-i`, `-I`, `-x`, `-F`)
- Design numeric-prefix support for normal-mode commands
