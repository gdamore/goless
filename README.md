<img src="site/static/assets/logos/goless-light-transparent.png" style="float: right" width="240" alt="logo"/>

# goless

[![Docs](https://img.shields.io/badge/godoc-reference-blue.svg?label=&logo=go)](https://pkg.go.dev/github.com/gdamore/goless)
[![Linux](https://img.shields.io/github/actions/workflow/status/gdamore/goless/linux.yaml?branch=main&logoColor=grey&logo=linux&label=)](https://github.com/gdamore/goless/actions/workflows/linux.yaml)
[![macOS](https://img.shields.io/github/actions/workflow/status/gdamore/goless/macos.yaml?branch=main&logoColor=grey&logo=apple&label=)](https://github.com/gdamore/goless/actions/workflows/macos.yaml)
[![Windows](https://custom-icon-badges.demolab.com/github/actions/workflow/status/gdamore/goless/windows.yaml?branch=main&logoColor=grey&logo=windows10&label=)](https://github.com/gdamore/goless/actions/workflows/windows.yaml)
[![Coverage](https://codecov.io/gh/gdamore/goless/branch/main/graph/badge.svg)](https://codecov.io/gh/gdamore/goless)

`goless` is a modern terminal text viewer and embeddable pure-Go pager core
for rendering textual content onto a `tcell.Screen` with behavior broadly
similar to `less`.

It is designed both for applications that need to display untrusted text safely
inside their own terminal UI and for the emerging standalone `goless` program,
without shell escapes or raw terminal control passthrough in the pager core.
It is implemented in pure Go, targets [`tcell`](https://github.com/gdamore/tcell),
and does not rely on cgo.

## Status

This project is still early-stage and not production-ready yet.

The public API is usable for experimentation, but compatibility is not stable
yet. Expect breaking changes while the library surface and internal model are
still settling.

## Documentation Site

Project documentation now lives in the Hugo site under `site/`.

A preview of the site is [available](https://gdamore.github.io/goless), but
be advised it is very far from complete still.

The intended structure is:

- a user manual for the standalone `goless` program
- a developer guide for applications embedding the pager library

To preview the site locally:

```bash
hugo server --source site
```

## What It Does Today

- Parses ANSI/ECMA-48 SGR styling and applies it through `tcell`
- Parses OSC 8 hyperlinks in `RenderHybrid` and `RenderPresentation`, with embedder-controlled live link policy
- Sanitizes unsupported escape/control sequences instead of letting them affect
  the host terminal
- Tracks Unicode grapheme clusters with `uniseg`
- Supports both horizontal scrolling and soft wrap
- Supports forward and reverse literal search with repeat-search
- Supports line jumps, follow mode, help overlay, and a status bar
- Exposes a reusable `Pager` API for embedding in other Go programs
- Includes a small full-screen standalone pager in `cmd/goless`

## Security Model

`goless` treats input as data, not terminal instructions.

- Supported SGR sequences affect only internal style state
- Unsupported or malformed escape/control sequences are either rendered
  visibly or hidden, depending on `RenderMode`
- Input is rendered only through `tcell`; sequences are not passed through to
  the host terminal
- Parsed OSC 8 hyperlinks are inert by default. An embedder must opt in with
  `HyperlinkHandler` before content becomes a live link.
- The pager core does not expose shell escape support or arbitrary subprocess
  execution
- The standalone `goless` program can launch `$EDITOR` for the current file
  with `v`; `-secure` disables that command

That is the core contract of the package: applications can display hostile or
arbitrary text without handing terminal control to the input stream.

### Hyperlink Security

OSC 8 hyperlinks deserve separate attention. A source can display text that
looks like one destination while the actual hyperlink target points somewhere
else.

`goless` therefore does not turn parsed OSC 8 sequences into live links on its
own. Instead, embedders are expected to make an explicit policy decision with
`HyperlinkHandler`.

That handler receives:

- the original target URL
- the optional OSC 8 `id=...`
- the full linked display text
- the base rendered style

From there, the application can:

- keep the link inert
- allow it to go live
- rewrite the target, for example to strip tracking parameters or route through
  a safe interstitial
- restyle the linked span

The intended secure default is conservative: if an application renders
untrusted content and has not made an explicit trust decision, it should leave
links inert or visibly tagged.

## Installation

```bash
go get github.com/gdamore/goless
```

Current module metadata targets Go `1.25`.

## Quick Start

```go
package main

import (
	"strings"

	"github.com/gdamore/goless"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

func main() {
	pager := goless.New(
		goless.WithWrapMode(goless.NoWrap),
		goless.WithKeyGroup(goless.LessKeyGroup),
		goless.WithRenderMode(goless.RenderHybrid),
		goless.WithShowStatus(true),
		goless.WithChrome(goless.Chrome{
			Title: "Example",
			Frame: goless.RoundedFrame(),
		}),
	)

	_, _ = pager.ReadFrom(strings.NewReader("hello\nworld\n"))
	pager.Flush()
	pager.SetSize(80, 24)

	term := vt.NewMockTerm(vt.MockOptSize{X: 80, Y: 24})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		return
	}
	if err := screen.Init(); err != nil {
		return
	}
	defer screen.Fini()
	screen.EnableMouse(tcell.MouseButtonEvents)

	pager.Draw(screen)
	_ = pager.SearchForward("world")
	pager.GoBottom()
	pager.Draw(screen)
}
```

The normal embedding model is:

1. Construct a `Pager` with `goless.New`.
2. Feed content with `Append`, `AppendString`, `ReadFrom`, or replace existing content with `ReloadFrom`.
3. Call `Flush` when input is complete or when you want incomplete parser state
   finalized.
4. Size with `SetSize`.
5. Render with `Draw`.
6. If you want wheel input without taking over click-drag selection, call `screen.EnableMouse(tcell.MouseButtonEvents)`.
7. Drive interaction through `HandleKey`, `HandleMouse`, or direct method calls.

## Public API Shape

The current exported `Pager` API is controller-oriented.

- Construction: `New(opts ...Option)`
- Runtime reconfiguration: `Configure(opts ...RuntimeOption)`
- Content loading: `Append`, `AppendString`, `ReadFrom`, `ReloadFrom`, `Flush`, `Len`
- Rendering: `SetSize`, `Draw`, `Refresh`
- Input integration: `HandleKey`, `HandleMouse`
- Navigation: `ScrollUp`, `ScrollDown`, `ScrollLeft`, `ScrollRight`, `HalfPageUp`,
  `HalfPageDown`, `PageUp`, `PageDown`, `GoTop`, `GoBottom`, `GoPercent`, `JumpToLine`
- Mode control: `ToggleWrap`, `SetWrapMode`, `WrapMode`, `Follow`, `StopFollow`, `Following`
- Search: `SearchForward`, `SearchBackward`, `SearchNext`, `SearchPrev`,
  `SearchForwardWithCase`, `SearchBackwardWithCase`, `SetSearchCaseMode`,
  `SearchCaseMode`, `SetSearchMode`, `SearchMode`, `CycleSearchCaseMode`,
  `CycleSearchMode`, `SearchState`, `ClearSearch`
- View state: `Position`
  `Position.Row` and `Position.Column` are 1-based logical coordinates when
  content is present, so `(1,1)` is the start of the first logical line

The main constructor/runtime options are available both as explicit `With...`
helpers and, for construction compatibility, through `Config` passed to `New`.

Common options are:

- `WrapMode`: `NoWrap` or `SoftWrap`
- `SearchCase`: `SearchSmartCase`, `SearchCaseSensitive`, or
  `SearchCaseInsensitive`
- `SearchMode`: `SearchSubstring`, `SearchWholeWord`, or `SearchRegex`
- `SqueezeBlankLines`: collapse consecutive empty logical lines into one visible line at view time
- `LineNumbers`: enable an adaptive line-number gutter
- `HeaderLines`: pin the first N logical lines at the top of the viewport
- `HeaderColumns`: pin the first N display columns at the left edge of the viewport
- `Theme`: remap content default colors and ANSI 0-15 without affecting chrome
- `Visualization`: show tabs, line endings, carriage returns, and EOF with pager-added markers
- `HyperlinkHandler`: inspect OSC 8 links, decide whether they go live,
  rewrite targets, and restyle spans
- `RenderMode`: `RenderHybrid`, `RenderLiteral`, or `RenderPresentation`
- `KeyGroup`: `LessKeyGroup` or `EmptyKeyGroup`
- `UnbindKeys` and `KeyBindings`: remove or prepend bindings in normal, help,
  or prompt contexts
- `Chrome`: optional frame/title styling plus title alignment and status/prompt style slots; frame glyphs are rendered exactly as configured, including intentional empty strings
- `Chrome.LineNumberStyle`: optional style for the adaptive line-number gutter
- `Chrome.HeaderStyle`: optional style for fixed header rows and columns
- `Text`: override help text, status text, prompt text, and UI strings

Runtime-safe `With...` options can be applied either through `Pager.Configure`
or through the existing convenience setters such as `SetTheme`, `SetChrome`,
and `SetHeaderLines`.

For OSC 8 specifically:

- parsed links are inert unless `HyperlinkHandler` opts into `Live`
- `HyperlinkHandler` sees both the displayed text and the actual target
- this is intended to steer embedders toward explicit, auditable link policy
  instead of silently trusting source-provided hyperlinks

By default, literal search uses smart-case behavior:

- lowercase queries search case-insensitively
- queries containing uppercase runes search case-sensitively

The built-in pager UI exposes search mode controls directly:

- `F2` in the bundled less-like key group cycles `auto -> case -> nocase`
- `F3` in the bundled less-like key group cycles `sub -> word -> regex`
- the search prompt always shows the current search mode
- the status bar shows search mode when a search is active or the search settings are non-default
- when otherwise idle, the left side of the status bar shows a subtle help hint: `F1 Help`
  Embedders can replace it with `Text.StatusHelpHint` or suppress it with `Text.HideStatusHelpHint`
- the right side of the status bar shows row and column as `current/total`
  These are 1-based logical coordinates rather than wrapped visual row numbers
- the built-in status bar reserves an EOF slot and shows `∎` when the end of the document is visible
- the right side of the status bar adds contextual wrap/scroll glyphs such as `↪` and `⇆`
- `:number` / `:nonumber` control line numbers
- `:wrap` / `:nowrap` control soft wrapping
- `:markers` / `:nomarkers` control hidden-character markers
- `:squeeze` / `:nosqueeze` control adjacent blank-line collapsing
- `:tabs <n>` sets tab width
- `:pin [rows=<n>] [cols=<n>]` pins the top rows and/or left columns
- `:match [auto|nocase|case] [sub|word|regex]` controls search case and matching
- `:help` opens the built-in help overlay
- invalid regexes stay in the search prompt and are marked visibly until fixed

`SqueezeBlankLines` is a view-time policy: raw input stays unchanged and
consecutive blank lines may be rendered as a single visible blank line, but
logical line numbering remains source-based for `Position()`, line numbers, and
`JumpToLine`.

Embedders are not locked to the bundled keys. They can:

- start from `EmptyKeyGroup`
- remove exact bundled keys with `UnbindKeys`
- prepend custom bindings with `KeyBindings`
- still reserve keys before pager dispatch with `CaptureKey`

`SearchState` exposes the current committed or preview search query, direction,
mode, case handling, match count/current position, and any regex compile error.

For host chrome integration:

- `Text.StatusLine` can replace the full left/right status bar text using
  `StatusInfo`
- `Text.PromptLine` can replace the full built-in prompt text using `PromptInfo`
- `Chrome.StatusStyle`, `Chrome.PromptStyle`, and `Chrome.PromptErrorStyle`
  can restyle the built-in bottom bar without replacing pager rendering
- `Theme` only affects document content. Explicit RGB colors and indexed colors
  above 15 are preserved exactly.
- In `Theme`, zero means "leave this mapping alone"; `color.Reset` means
  "explicitly use the terminal default here".
- `Visualization` controls hidden-character overlays separately from `Theme`.
- Built-in preset bundles are available as `DarkPreset`, `LightPreset`,
  `PlainPreset`, and `PrettyPreset`

## Render Modes

- `RenderHybrid`
  Supported styling is applied. Unsupported sequences are shown visibly.
  Parsed OSC 8 links are available to the embedder hyperlink handler.
- `RenderLiteral`
  Escape and control sequences are shown literally and do not affect styling.
- `RenderPresentation`
  Supported styling is applied. Unsupported sequences are consumed and hidden.
  Parsed OSC 8 links are available to the embedder hyperlink handler.

## Standalone Program

The repository includes a standalone program in `cmd/goless`, which
has enough functionality to be used as your usual `$PAGER`.

```bash
go run ./cmd/goless -- file.txt
```

It also accepts less-style startup directives and multiple files:

```bash
go run ./cmd/goless +42 -- a.txt b.txt
go run ./cmd/goless +/needle -- a.txt b.txt
```

It can also read from stdin:

```bash
printf 'hello\n\033[31mworld\033[0m\n' | go run ./cmd/goless
```

You can name stdin explicitly with `-`, including alongside files:

```bash
printf 'hello\n' | go run ./cmd/goless -- -
printf 'hello\n' | go run ./cmd/goless -- a.txt - b.txt
```

If `stdout` is not a terminal, `goless` skips the full-screen UI and copies the
selected input to `stdout` unchanged.

Program flags:

- `-?` or `--help` to print usage and exit
- `--version` to print the program version and exit
- `-e` or `--quit-at-eof` to quit on the first forward command at EOF
- `-E` or `--QUIT-AT-EOF` to quit when EOF is already visible on screen
- `-F` to quit immediately when the current input fits on one screen
- `-N` to enable line numbers
- `-R` accepted as a less-compatibility no-op
- `-S` accepted as a less-compatibility no-op because no-wrap is already the default
- `-secure` to disable standalone commands that launch external programs
- `-i` for smart-case search behavior
- `-I` for case-insensitive search behavior
- `--license` to open the bundled Apache license, or print it when stdout is not a terminal
- `--default-config` to print the built-in JSON config to stdout
- `--mouse` to capture mouse button and wheel events in the standalone program
- `--no-mouse` to disable mouse button and wheel capture in the standalone program
- `-config path` to load a specific JSON config file instead of the default per-user path
- `-x n` to set tab width
- `-theme dark|light|plain|pretty`
- `-chrome auto|none|single|rounded`
- `--markers` / `--no-markers` to show or hide hidden-character markers
- `--live-links` to enable live OSC 8 hyperlinks in the standalone program
- `--literal` / `--no-literal` to show escape sequences literally or interpret supported escapes
- `-s` or `--squeeze` to collapse repeated blank lines in the current view
- `-title text`
- optional `+line` or `+/pattern` startup directive before paths
- `-` as an explicit stdin path

When present, `goless` also loads per-user configuration from
`goless/config.json` under the OS config directory returned by
`os.UserConfigDir()`; on most Linux systems that means
`$XDG_CONFIG_HOME/goless/config.json` or `~/.config/goless/config.json`.
On macOS that is typically `~/Library/Application Support/goless/config.json`.
On Windows that is typically `%AppData%\goless\config.json`.

Config selection precedence is:

1. `-config path`
2. `GOLESS_CONFIG`
3. the default per-user config path

The initial config schema is intentionally small:

```json
{
  "theme": "pretty",
  "hidden": false,
  "line-numbers": false,
  "live-links": false,
  "mouse": true,
  "secure": false
}
```

The JSON config still uses the `"hidden"` field name; the CLI exposes the same
setting as `--markers` / `--no-markers`.

Use `goless --default-config` to print that built-in config and redirect it into
`goless/config.json` or another starter file.

CLI flags still take precedence over config values, so
`goless -config ./alt.json -theme dark file.txt` overrides the selected config
file's `"theme"` value for that invocation, and `goless --mouse file.txt`
temporarily enables mouse capture even when the config sets `"mouse": false`.

Set `"mouse": false` or pass `--no-mouse` if you want the
standalone program to leave terminal text selection and native scrolling alone.

The default key group is intentionally less-like. Common bindings include:

- `q` or `Esc` to quit
- `j`/`k` for one-line scrolling
- `Up`/`Down` for coarse vertical step scrolling
- `Shift-Up`/`Shift-Down` for fine one-line scrolling
- `u`/`d` or `Ctrl-U`/`Ctrl-D` for half-page up/down
- `space` for page down and `b`/`w` for page up, plus `Ctrl-B` and `Alt-V`
- `g`/`G` for top/bottom
- `W` to toggle wrap
- `r` or `Ctrl-L` to repaint the screen
- `Ctrl-Z` to suspend and resume the terminal session
- `s` to open the standalone save prompt for the current content set
- `v` to open the current file in `$EDITOR` at the current line unless `-secure` is set
- `/` and `?` to search forward/backward
- `n` and `N` to repeat search
- `F2` to cycle search case mode in the bundled key group
- `F3` to cycle substring, whole-word, and regex matching in the bundled key group
- `F4` to cycle visual themes in the standalone program
- `F5` to toggle hidden-character markers in the standalone program
- `:` then a number to jump to a line
- `:50%` to jump near the middle of the document
- `:next` / `:prev` to move through a multi-file session
- `:first`, `:last`, `:x`, and `:file` for file-session control
- `R` or `:reload` to reload the current file/input while keeping the current viewport when possible
- `:save <path>` / `:write <path>` / `:w <path>` to save either the full current content set or the visible viewport; `s` opens a prompt for the same modes, and `-secure` disables save
- `:license` to show the bundled Apache license in an overlay
- `:Q` as an additional quit command
- `=` or `Ctrl-G` to show current file/session status in the standalone program
- `:number`                              show line numbers
- `:nonumber`                            hide line numbers
- `:wrap`                                soft-wrap long lines
- `:nowrap`                              disable soft wrapping
- `:markers`                             show hidden-character markers
- `:nomarkers`                           hide hidden-character markers
- `:squeeze`                             collapse adjacent blank lines
- `:nosqueeze`                           keep blank lines
- `:tabs <n>`                           set tab width
- `:pin [rows=<n>] [cols=<n>]`          pin the top rows and/or left columns
- `:match [auto|nocase|case] [sub|word|regex]`  control search case and matching
- `:help`                               show the built-in help overlay
- `F` to enable follow mode
- `Ctrl-X` or `Ctrl-C` to stop following without quitting
- `F1` to toggle help

## Hardening and Performance

The repository now includes:

- fuzzing for parser chunking behavior
- fuzzing for incremental document ingestion behavior
- benchmarks for parser throughput, document append, layout rebuild, and search

Those checks are there to keep the untrusted-input path honest and to make
performance work measurable as the library evolves.

## Current Limitations

- API compatibility is not stable yet
- Regex search is available, but the search feature set is intentionally lightweight
- The public package exposes a pager core, not a full standalone `less`
  replacement
- The standalone program is intentionally small and does not aim to replicate
  every `less` behavior
- See [LESS_COMPATIBILITY.md](LESS_COMPATIBILITY.md) for the current `less`
  compatibility audit and selected follow-up scope

## Development

Useful commands while working on the repository:

```bash
go test ./...
go test ./internal/ansi -fuzz FuzzParserChunkedMatchesSingleWrite -fuzztime=3s
go test ./internal/model -fuzz FuzzDocumentChunkedMatchesSingleAppend -fuzztime=3s
go test ./internal/ansi ./internal/model ./internal/layout ./internal/view -run '^$' -bench Benchmark -benchmem
```

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE).
