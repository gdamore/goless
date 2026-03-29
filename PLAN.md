# goless Plan

## Goal

Build an embeddable pure-Go library for viewing textual content on a `tcell.Screen`
with behavior similar to `less`, while remaining secure and suitable for use inside
other Go programs.

The library should:

- Render styled text, including SGR/ECMA-48 color and text attributes.
- Support full Unicode grapheme cluster handling.
- Consume content from `io.Reader` and other adapters.
- Support forward and reverse searching with match highlighting.
- Sanitize non-display control/escape sequences so input cannot manipulate the host terminal.
- Support either horizontal scrolling or soft wrapping.
- Expose a reusable library API and a small demo program.

## Non-Goals

- No shell escapes, editor launches, subprocess execution, or pager-style `!` commands.
- No dependence on external pager binaries.
- No direct pass-through of terminal escape sequences to the host terminal.
- No requirement to preserve byte-for-byte terminal behavior of GNU `less`.

## Core Design Principles

1. Security first: parse input into an internal model and render only through `tcell`.
2. Separate parsing, layout, and rendering. Do not mix ANSI parsing with screen painting.
3. Use a real byte-oriented state machine for escape parsing and UTF-8 decoding.
4. Treat grapheme clusters as the smallest user-visible text unit.
5. Measure viewport layout in screen cells, not bytes or runes.
6. Keep the library embeddable: avoid a monolithic blocking API where possible.
7. Prefer explicit internal abstractions over guessing behavior from concrete source types.
8. Keep logical text order separate from visual display order so future bidi support remains feasible.

## Functional Requirements

### Rendering

- Support ECMA-48 SGR attributes:
  - foreground/background basic colors
  - bright colors
  - 256-color palette
  - 24-bit color
  - bold
  - dim
  - underline
  - reverse
  - blink if representable through `tcell`
  - italic if representable through `tcell`
  - reset semantics
- Translate parsed style state into `tcell.Style`.
- Ignore or sanitize non-SGR escape/control sequences so they cannot affect the host terminal.
- Render tabs using configurable tab width.
- Render newlines as line boundaries.
- Render other control bytes visibly using a consistent replacement policy.

### Unicode

- Use `github.com/rivo/uniseg` for grapheme segmentation.
- Preserve combining marks and zero-width joiner sequences as single grapheme clusters.
- Handle wide-cell graphemes correctly when computing screen positions.
- Never split a grapheme cluster during wrapping, highlighting, or horizontal scrolling.
- Keep text, style ranges, and search state in logical order rather than baking in left-to-right display assumptions.
- Treat visual reordering as a layout concern so future Hebrew/Arabic bidi support can be added without rewriting the document model.

### Content Sources

- Accept `io.Reader` as a baseline input type.
- Provide adapters for:
  - `string`
  - `[]byte`
  - `*os.File`
  - `bytes.Reader`
  - `strings.Reader`
- Support optional source length when available.
- Note: the Go standard library does not define a general `io.Reader` interface that also exposes size/length.
- Internally normalize all sources behind a project-defined source abstraction.

### Navigation

- Vertical scrolling by line/visual row.
- Optional horizontal scrolling in no-wrap mode.
- Optional soft wrapping mode.
- Jump-to-top / jump-to-bottom.
- Page up / page down.
- Search next / previous result.

### Search

- Forward search with `/`.
- Reverse search with `?`.
- Repeat search with `n` / `N`.
- Highlight all visible matches, or at minimum the active match and visible matches on screen.
- Search should operate on sanitized visible text, not raw escape sequences.
- Search should work across styled content without requiring style resets.

### Status Bar

- Bottom status bar showing:
  - current position in content
  - total content length if known
  - current mode (`WRAP` or `SCROLL`)
  - active search query if any
- Position reporting should be meaningful even when total size is unknown.

### Demo Program

- Provide a small `main` package that behaves similarly to `less` within project limits.
- Read from stdin or file arguments.
- Expose a subset of familiar keys.
- Avoid any feature that would compromise embeddability or security.

## Internal Model

The implementation should keep distinct coordinate systems:

- Byte offsets in the original input.
- Rune offsets in decoded text.
- Grapheme cluster indices in visible logical text.
- Screen cell positions for rendering.

These must not be conflated. Search, wrapping, highlighting, and viewport motion will
be incorrect if these are collapsed prematurely.

Recommended storage strategy:

- Do not use a rope as the primary document representation.
- A rope mainly optimizes mid-document edits, which are not a primary requirement here.
- Prefer an append-only segmented model optimized for:
  - immutable content already received
  - efficient appends at end of file
  - reverse search
  - tailing/log-follow use cases
  - stable offset mapping

## Data Model

Use four layers:

1. Byte store
   - Append-only chunked storage, for example fixed-size slabs.
   - Primary authority for original bytes and absolute byte offsets.
   - Supports efficient append without repeated full-buffer copying.
   - May be memory-backed initially, with file-backed spooling as an implementation option later.
2. Incremental parser state
   - Maintains the current escape-parser state, UTF-8 partial state, and active style state.
   - Continues across append boundaries and chunk boundaries.
   - Produces normalized visible output incrementally.
3. Logical document index
   - Stores logical lines derived from parsed visible content.
   - Each line carries text, style runs, grapheme boundaries, and source byte range metadata.
   - Appends only affect the tail line and newly completed lines.
4. Layout cache
   - Derived from logical lines.
   - Parameterized by viewport width, wrap mode, and tab width.
   - Disposable and rebuildable on resize or mode change.

This fits the workload better than either a single giant `[]byte` or a rope.

## Byte Store

Recommended internal interface:

```go
type ByteStore interface {
    Append(p []byte) (start int64, end int64)
    ReadAt(p []byte, off int64) (int, error)
    Len() int64
}
```

Implementation notes:

- Start with an in-memory chunked store.
- Use chunk sizes large enough to avoid overhead but small enough to avoid pathological copy costs.
- Later add a spill-to-disk implementation if large or unbounded streams become important.
- Keep offsets absolute so search results, status reporting, and tailing behavior remain stable.

Why this instead of a rope:

- Simpler implementation.
- Better fit for append-only growth.
- Easier incremental parsing across chunk boundaries.
- Easier reverse search and byte-offset accounting.
- No need to optimize random inserts or deletes in the middle of the document.

## Logical Document Index

The logical document should be derived metadata, not the byte store itself.

Each logical line should retain enough information to support rendering, searching,
and status reporting without reparsing the entire document:

```go
type Line struct {
    ByteStart int64
    ByteEnd   int64
    Text      string
    Graphemes []int
    Styles    []StyleRun
}
```

Notes:

- `Text` is sanitized visible text, not raw input bytes.
- `Graphemes` should record grapheme boundaries or equivalent derived indexing data.
- `Styles` should map visible text ranges to the effective display style.
- Additional cached width information may be stored if profiling shows it is useful.

Do not make graphemes the primary storage layer. They are derived metadata used for
search mapping, wrapping, and safe viewport math.

## Append and Tail Semantics

Appends are a first-class requirement.

Behavior:

- New bytes append to the `ByteStore`.
- The parser resumes from its previous state rather than restarting from byte zero.
- Parsed output extends the current tail line or creates additional lines.
- Only affected tail layout/search caches are invalidated.

This should support:

- streaming input from `io.Reader`
- reading large files incrementally
- `tail -f` style log viewing

Future option:

- Add bounded-retention mode for long-running tails, such as keeping only the last N bytes or N lines.
- If this is added, preserve absolute positions even when old chunks are dropped.

## Proposed Pipeline

1. Source adapter
   - Accepts an input source.
   - Exposes streaming reads and optional known length.
   - Feeds an append-only internal byte store.
2. Decoder and sanitizer
   - Reads appended bytes from the byte store.
   - Parses ECMA-48 sequences with a real state machine.
   - Decodes UTF-8 as part of the same byte-stream parser instead of pre-tokenizing by rune.
   - Emits styled visible text plus sanitized replacements for disallowed controls.
3. Logical document builder
   - Produces logical lines composed of grapheme-aware styled spans.
   - Retains byte-range mapping back to source content.
   - Updates incrementally as new bytes are appended.
4. Layout engine
   - Expands tabs.
   - Produces visual rows according to screen width and wrap mode.
   - Computes cell widths safely for wide graphemes.
5. Viewport/controller
   - Tracks current vertical offset.
   - Tracks horizontal cell offset in no-wrap mode.
   - Handles search state and active match.
6. Renderer
   - Paints content rows into `tcell.Screen`.
   - Paints search highlights and status bar.

## Source Abstraction

Define an internal source interface in terms of append/read behavior rather than relying
on ad hoc type assertions in the core:

```go
type Source interface {
    Fill(store ByteStore) error
    Size() (int64, bool)
}
```

Notes:

- Not every input can satisfy random access directly.
- For plain `io.Reader`, the adapter should append into the `ByteStore` as bytes arrive.
- For `*os.File` or seekable readers, the adapter may still choose chunked ingest to keep the rest of the system uniform.
- `Size() (int64, bool)` allows callers to distinguish unknown size from a true zero length.
- Additional constructors can hide the adaptation details:
  - `NewFromReader(io.Reader)`
  - `NewFromFile(*os.File)`
  - `NewFromBytes([]byte)`
  - `NewFromString(string)`
- For tailing use cases, add a source or controller mode that keeps reading and appending after initial display.

## Escape and Control Handling

Only SGR styling escapes should affect styling state. Other ECMA-48 control families
must be recognized and neutralized.

Parser model:

- Implement the parser as an explicit state machine over bytes.
- Keep separate states for at least:
  - initial/default text
  - escape introducer
  - CSI
  - OSC
  - generic string bodies such as DCS/APC/PM/SOS
  - UTF-8 continuation handling
- Do not attempt to parse by splitting strings or decoding runes first; escape recognition happens at the byte level.
- Use the `tcell` VT emulator as the reference implementation shape, especially the state-function approach in [emulate.go](/Users/garrettdamore/Projects/tcell/vt/emulate.go).
- The mock terminal in [mock.go](/Users/garrettdamore/Projects/tcell/vt/mock.go) is also relevant as a proven consumer/test harness around that parser model.
- For initial implementation, it is acceptable to copy/adapt parser logic from the local `tcell` VT emulator and then trim behavior down to the secure pager use case.

Minimum parser support:

- ESC
- CSI
- OSC
- DCS
- APC
- PM
- SOS

Handling rules:

- SGR sequences update style state.
- Other recognized escape/control sequences are consumed and dropped or replaced with a visible marker.
- Malformed or unterminated sequences must not leak raw control effects to the terminal.
- OSC 8 and similar sequences must be fully consumed and neutralized.
- Invalid UTF-8 subsequences must be handled deliberately and must not desynchronize the escape parser.
- Unicode grapheme processing with `uniseg` occurs after byte-stream decoding and sanitization, not inside the escape state machine.

Replacement policy for controls:

- Newline becomes a logical line break.
- Tab expands according to configured tab width.
- Other C0/C1 controls render visibly using a project-defined glyph policy.
- Consider an option to render `ESC` visibly as a Unicode control-picture-style glyph.

## Wrapping and Horizontal Scrolling

Expose a top-level mode:

```go
type WrapMode int

const (
    NoWrap WrapMode = iota
    SoftWrap
)
```

Semantics:

- `NoWrap`
  - Preserve logical lines.
  - Track horizontal offset in screen cells.
  - Reveal search matches by adjusting both vertical and horizontal offsets.
- `SoftWrap`
  - Wrap logical lines into visual rows.
  - Wrap only on grapheme boundaries.
  - Prefer cell-aware wrapping after tab expansion.
  - Reveal search matches by scrolling to the visual row containing the match.

Requirements:

- Horizontal offsets must never split a rendered grapheme cluster.
- Status bar should show the active mode.
- The demo should support toggling mode at runtime.

## Search Semantics

Initial recommendation:

- Literal substring search on sanitized visible text.
- Case-sensitive by default.
- Support forward and reverse direction.
- Support repeat commands with direction-aware behavior.

Data needed for search:

- Logical text for matching.
- Mapping from match positions back to logical lines/grapheme ranges.
- Layout mapping from logical matches to visible rows and cells.

Future-compatible extension points:

- Smart-case search.
- Regex search.
- Whole-word search.

## Public API Direction

The project should expose a reusable viewer object rather than only a blocking pager loop.

Example shape:

```go
type Viewer struct {
    // internal state
}

type Config struct {
    TabWidth int
    WrapMode WrapMode
    ShowStatus bool
    ControlPolicy ControlPolicy
}

func New(src Source, cfg Config) (*Viewer, error)
func NewFromReader(r io.Reader, cfg Config) (*Viewer, error)

func (v *Viewer) SetSize(width, height int)
func (v *Viewer) Draw(s tcell.Screen)
func (v *Viewer) HandleKey(ev *tcell.EventKey) bool
func (v *Viewer) SearchForward(query string) error
func (v *Viewer) SearchBackward(query string) error
func (v *Viewer) SetWrapMode(mode WrapMode)
func (v *Viewer) Position() Position
```

Embeddability goals:

- Callers can own the event loop if desired.
- The viewer can also offer a helper `Run(screen tcell.Screen)` for convenience.
- Avoid exposing raw parsing internals unless they are intentionally reusable.

## Suggested Package Layout

```text
/cmd/goless-demo        demo pager program
/internal/ansi          ECMA-48 parsing and style state
/internal/document      source adapters and logical document model
/internal/layout        grapheme-aware wrapping and cell measurement
/internal/search        search indexing and match navigation
/internal/render        tcell rendering helpers
/viewer                 public API
```

Alternative:

- Keep everything in a single root package initially if speed matters.
- Split into internal packages once interfaces stabilize.

Because the repository is new, starting with a single public package plus a small set of
internal packages is reasonable.

## Milestones

### Milestone 1: Repository Bootstrap

- Initialize Go module.
- Add package skeleton.
- Add dependency on `tcell` and `uniseg`.
- Add a tiny demo that opens a screen and renders plain text.
- Add CI for `go test`.

### Milestone 2: Safe Styled Text Parsing

- Implement ECMA-48 parser with style-state tracking using an explicit byte-state machine.
- Start by copying/adapting the parser structure already implemented in `tcell` VT emulation rather than designing a weaker ad hoc parser.
- Support SGR color and attributes.
- Sanitize non-SGR controls.
- Add unit tests for normal, malformed, and adversarial sequences.

### Milestone 3: Unicode-Aware Document Model

- Build logical lines from sanitized parsed content.
- Integrate `uniseg`.
- Track grapheme widths and line boundaries.
- Add tests for combining marks, emoji, ZWJ sequences, and East Asian wide text.

### Milestone 4: Viewport and Layout

- Implement soft wrap and no-wrap layouts.
- Implement vertical and horizontal scrolling.
- Add status bar.
- Add resize handling.

### Milestone 5: Search

- Add forward and reverse literal search.
- Track active match.
- Highlight visible matches.
- Add repeat-search behavior.

### Milestone 6: Demo Pager

- Add CLI handling for stdin/files.
- Map a small set of `less`-style keys.
- Support search prompt and wrap toggle.

### Milestone 7: Hardening

- Fuzz parser and sanitizer.
- Benchmark on large inputs.
- Improve incremental layout and search performance.
- Document security model and unsupported sequences.

## Testing Strategy

Unit tests:

- ANSI/ECMA-48 parser behavior.
- Style translation to `tcell.Style`.
- Grapheme segmentation.
- Cell-width calculation.
- Wrapping and horizontal scrolling boundaries.
- Search result mapping.

Fuzz tests:

- Escape sequence parser.
- Sanitizer.
- Search indexing over arbitrary Unicode input.

Golden-style tests:

- Render known inputs into an in-memory screen model.
- Verify visual rows, styles, and status bar output.

Integration tests:

- Drive a simulated viewer with key events.
- Verify navigation, search, and mode switching.

## Performance Considerations

- Avoid reparsing the entire document on every redraw.
- Cache logical lines and style spans after initial parse.
- Cache wrapped layout per width where feasible.
- Recompute only affected layout on resize or wrap toggle.
- Consider lazy indexing for very large inputs.

## Open Decisions

These should be settled early:

1. Should search be literal-only in v1, or should regex support exist from the start?
2. Should all visible matches be highlighted, or only matches on the current screen?
3. Should unknown-size streaming sources be fully buffered before interaction, or should interaction begin while buffering continues?
4. What visible glyph policy should be used for control characters?
5. Should the public package expose parser primitives, or keep them internal?

## Immediate Next Steps

1. Initialize the Go module and choose the public package path/name.
2. Create the `viewer.Config`, `WrapMode`, and source-constructor skeletons.
3. Implement the source abstraction and buffering adapter for `io.Reader`.
4. Build the ANSI/SGR parser with tests before any full-screen UI work.
5. Add a minimal demo that renders sanitized plain content through `tcell`.
