# goless

`goless` is an embeddable pure-Go pager core for rendering textual content onto
a `tcell.Screen` with behavior broadly similar to `less`.

It is designed for applications that need to display untrusted text safely
inside their own terminal UI, without shell escapes, subprocess hooks, or raw
terminal control passthrough.

## Status

This project is still early-stage and not production-ready yet.

The public API is usable for experimentation, but compatibility is not stable
yet. Expect breaking changes while the library surface and internal model are
still settling.

## What It Does Today

- Parses ANSI/ECMA-48 SGR styling and applies it through `tcell`
- Sanitizes unsupported escape/control sequences instead of letting them affect
  the host terminal
- Tracks Unicode grapheme clusters with `uniseg`
- Supports both horizontal scrolling and soft wrap
- Supports forward and reverse literal search with repeat-search
- Supports line jumps, follow mode, help overlay, and a status bar
- Exposes a reusable `Pager` API for embedding in other Go programs
- Includes a small full-screen demo pager in `cmd/goless-demo`

## Security Model

`goless` treats input as data, not terminal instructions.

- Supported SGR sequences affect only internal style state
- Unsupported or malformed escape/control sequences are either rendered
  visibly or hidden, depending on `RenderMode`
- Input is rendered only through `tcell`; sequences are not passed through to
  the host terminal
- There is no shell escape support, editor launch support, or subprocess
  execution support

That is the core contract of the package: applications can display hostile or
arbitrary text without handing terminal control to the input stream.

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
	pager := goless.New(goless.Config{
		WrapMode:   goless.NoWrap,
		KeyGroup:   goless.LessKeyGroup,
		RenderMode: goless.RenderHybrid,
		ShowStatus: true,
		Chrome: goless.Chrome{
			Title: "Example",
			Frame: goless.RoundedFrame(),
		},
	})

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

	pager.Draw(screen)
	_ = pager.SearchForward("world")
	pager.GoBottom()
	pager.Draw(screen)
}
```

The normal embedding model is:

1. Construct a `Pager` with `goless.New`.
2. Feed content with `Append`, `AppendString`, or `ReadFrom`.
3. Call `Flush` when input is complete or when you want incomplete parser state
   finalized.
4. Size with `SetSize`.
5. Render with `Draw`.
6. Drive interaction through `HandleKey` or direct method calls.

## Public API Shape

The current exported `Pager` API is controller-oriented.

- Content loading: `Append`, `AppendString`, `ReadFrom`, `Flush`, `Len`
- Rendering: `SetSize`, `Draw`, `Refresh`
- Key-driven integration: `HandleKey`
- Navigation: `ScrollUp`, `ScrollDown`, `ScrollLeft`, `ScrollRight`, `PageUp`,
  `PageDown`, `GoTop`, `GoBottom`, `JumpToLine`
- Mode control: `ToggleWrap`, `SetWrapMode`, `WrapMode`, `Follow`, `Following`
- Search: `SearchForward`, `SearchBackward`, `SearchNext`, `SearchPrev`,
  `SearchForwardWithCase`, `SearchBackwardWithCase`, `SetSearchCaseMode`,
  `SearchCaseMode`, `SetSearchWordMode`, `SearchWordMode`,
  `CycleSearchCaseMode`, `CycleSearchWordMode`, `ClearSearch`
- View state: `Position`

The main config knobs are:

- `WrapMode`: `NoWrap` or `SoftWrap`
- `SearchCase`: `SearchSmartCase`, `SearchCaseSensitive`, or
  `SearchCaseInsensitive`
- `SearchWord`: `SearchSubstring` or `SearchWholeWord`
- `RenderMode`: `RenderHybrid`, `RenderLiteral`, or `RenderPresentation`
- `KeyGroup`: currently `LessKeyGroup`
- `Chrome`: optional frame/title styling
- `Text`: override help text, status text, and UI strings

By default, literal search uses smart-case behavior:

- lowercase queries search case-insensitively
- queries containing uppercase runes search case-sensitively

The built-in pager UI exposes search mode controls directly:

- `F2` in the bundled less-like key group cycles `smart -> case -> nocase`
- `F3` in the bundled less-like key group toggles `sub -> word`
- the current mode is shown in the status bar and search prompt
- `:set searchcase smart|case|nocase` is available as a fallback
- `:set searchword sub|word` is available as a fallback

Embedders are not locked to `F2` or `F3`. They can reserve bundled keys with
`CaptureKey` and drive mode changes explicitly through `CycleSearchCaseMode`,
`SetSearchCaseMode`, `CycleSearchWordMode`, or `SetSearchWordMode`.

## Render Modes

- `RenderHybrid`
  Supported styling is applied. Unsupported sequences are shown visibly.
- `RenderLiteral`
  Escape and control sequences are shown literally and do not affect styling.
- `RenderPresentation`
  Supported styling is applied. Unsupported sequences are consumed and hidden.

## Demo Program

The repository includes a small demo in `cmd/goless-demo`.

```bash
go run ./cmd/goless-demo -- file.txt
```

It can also read from stdin:

```bash
printf 'hello\n\033[31mworld\033[0m\n' | go run ./cmd/goless-demo
```

Demo flags:

- `-chrome none|single|rounded`
- `-render hybrid|literal|presentation`
- `-title text`

The default key group is intentionally less-like. Common bindings include:

- `q` or `Esc` to quit
- `j`/`k` or arrow keys to scroll
- `space`/`b` for page down/up
- `g`/`G` for top/bottom
- `w` to toggle wrap
- `/` and `?` to search forward/backward
- `n` and `N` to repeat search
- `F2` to cycle search case mode in the bundled key group
- `F3` to toggle substring vs whole-word matching in the bundled key group
- `:` then a number to jump to a line
- `:set searchcase smart|case|nocase` to set search mode directly
- `:set searchword sub|word` to set whole-word matching directly
- `F` to enable follow mode
- `H` or `F1` to toggle help

## Hardening and Performance

The repository now includes:

- fuzzing for parser chunking behavior
- fuzzing for incremental document ingestion behavior
- benchmarks for parser throughput, document append, layout rebuild, and search

Those checks are there to keep the untrusted-input path honest and to make
performance work measurable as the library evolves.

## Current Limitations

- API compatibility is not stable yet
- Search is currently literal only; there is no regex mode yet
- The public package exposes a pager core, not a full standalone `less`
  replacement
- The demo is intentionally small and does not aim to replicate every `less`
  behavior

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
