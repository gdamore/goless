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
- `:q`, `:Q`, and `:quit`
- `:n` and `:p` through the standalone program's command handler
- `:x` through the standalone program's command handler
- `:f` through the standalone program's `file` / `f` command alias
- `=` and `Ctrl-G` for current file/session status in the standalone program
- `+42` and `+/pattern` startup directives

Useful, but intentionally different:

- `:set ...` commands are custom pager commands, not `less` option toggles
- search mode and case mode are exposed directly with `F2`, `F3`, and `:set`

Notable gaps:

- no `ZZ` quit alias
- no `:e [file]` command to examine a new file
- no `R` hard reload behavior
- no support for marks, bracket matching, or tag navigation
- no file-spanning search commands such as `ESC-n` / `ESC-N`

### Keybindings

Compatible today:

- `q`, `Q`, `Esc`
- `j` / `k`, `y` / `e`, `Enter`, and arrow keys
- `Ctrl-Y`, `Ctrl-K`, `Ctrl-P`, `Ctrl-E`, `Ctrl-N`
- `space`, `f`, `b`, `Ctrl-B`, `Ctrl-F`, `Ctrl-V`, `Alt-v`
- `g`, `G`, `<`, `>`
- `/`, `?`, `n`, `N`
- `r`, `Ctrl-L` for repaint
- `F` for follow mode

Close, but thinner than `less`:

- horizontal navigation exists, but not under the usual `less` bindings

Current sequence limitation:

- the keymap only supports single `tcell.EventKey` bindings today
- `Alt-v` is supported as the pragmatic stand-in for `less`'s historical
  `Esc-v` backward-page behavior
- true multi-key prefixes such as literal `Esc` then `v` are intentionally not
  supported; they would require explicit prefix-state handling and `Esc`
  timeout/cancellation rules

### CLI Switches and Startup Syntax

Compatible today:

- `-?` / `--help`
- `-e` / `--quit-at-eof`
- `-E` / `--QUIT-AT-EOF`
- `-F` to quit immediately when the current input fits on one screen
- `-N` to enable line numbers
- `-R` as a compatibility no-op because the default rendering mode already
  aligns with the intended less-like behavior
- `-S` as a compatibility no-op because no-wrap is already the default mode
- `-i` / `-I` for smart-case and case-insensitive search behavior
- `-s` / `--squeeze` to collapse repeated blank lines in the current view
- `-x N` to set tab width
- `+line`
- `+/pattern`
- multiple files plus explicit `-` for stdin

Custom flags such as `-preset`, `-chrome`, `-hidden`, `-render`, and
`-live-links` are fine; they do not block familiarity as long as common `less`
habits also work.

## Recommended Changes

These are the changes most likely to improve familiarity without dragging the
project into full GNU `less` emulation.

### 1. Expand the bundled less-like key aliases

Status: partially complete.

Implemented:

- `Enter`, `e`, `Ctrl-E`, `Ctrl-N` for forward one line
- `Ctrl-J` for forward one line
- `y`, `Ctrl-Y`, `Ctrl-K`, `Ctrl-P` for backward one line
- `Ctrl-F`, `Ctrl-V` for page down
- `Ctrl-B` for page up
- `Alt-v` as the pragmatic stand-in for `Esc-v` page up
- `w` for backward-page behavior
- `r` and `Ctrl-L` for repaint
- `Q` as an additional quit key
- `<` goes to top of file
- `>` goes to bottom of file
- `h` opens and closes help
- `W` toggles wrap so `w` can stay less-compatible
- `R` reloads the current file/input in place

Still open:

- no additional single-key alias changes selected in this batch

Keep fine horizontal scrolling on the existing shifted-arrow bindings. This is a
better trade than spending two prominent `less` keys on a niche horizontal
motion variant.

### 2. Add the most common standalone program command aliases

Status: partially complete.

Implemented:

- `:Q`
- `:x`
- `=`
- `Ctrl-G`
- `:reload`
- `v` to launch `$EDITOR` for the current file, unless `-secure` is set

`=` and `Ctrl-G` report the same current-file/session status already available
through `:file`.

Still open:

- `:e <file>`

### 3. Add common CLI aliases with low behavioral risk

Status: partially complete.

Implemented:

- `-N` to enable line numbers
- `-i` to keep smart-case behavior explicit
- `-I` to force case-insensitive search
- `-s` / `--squeeze` to collapse repeated blank lines in the current view
- `-x N` to set tab width
- `-F` to quit if the current input already fits on one screen

`-S` is accepted as an alias for no-wrap mode even though that is
already the default. That keeps scripts and muscle memory working without
forcing users to know the current default.

`-R` is accepted as a compatibility no-op because the default rendering mode is
already the intended less-like behavior.

`-?` and `--help` are accepted as standard help entry points.

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
- save-to-file commands
- `LESSOPEN`, lesskey, and other external-process integration points
- raw control-character passthrough mode `-r`
- exact GNU `less` prompt formatting and option-toggling syntax

Those features either conflict with the security model or add substantial
surface area without materially improving the experience for most users.

## Suggested Follow-Up Issues

- Add standalone program `:e <file>` and any secure shell-command opt-ins
- Design numeric-prefix support for normal-mode commands
- Decide whether to add sequence support or keep keymaps single-event only
