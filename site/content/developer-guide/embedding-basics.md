+++
title = "Embedding Basics"
summary = "Create a pager, load content, size it, and draw it onto a tcell screen."
weight = 10
+++

This page answers the first practical integration question: "How do I get a
pager on screen?"

```go
func runPager(screen tcell.Screen, input io.Reader) error {
	pager := goless.New(
		goless.WithKeyGroup(goless.LessKeyGroup),
		goless.WithRenderMode(goless.RenderHybrid),
		goless.WithShowStatus(true),
		goless.WithChrome(goless.Chrome{
			Title: "Example",
			Frame: goless.RoundedFrame(),
		}),
	)

	if _, err := pager.ReadFrom(input); err != nil {
		return err
	}
	pager.Flush()

	width, height := screen.Size()
	pager.SetSize(width, height)
	pager.Draw(screen)
	return nil
}
```

The simplest useful embedding loop has four steps:

- load the document content
- size the pager to the region it should occupy
- draw it onto the host screen
- forward input and resize events back into the pager

## Loading Content

`goless` can ingest text in a few different ways.

- `ReadFrom` is the easiest path when you already have an `io.Reader`.
- `Append` and `AppendString` are useful when input arrives incrementally.
- `ReloadFrom` replaces the current document while keeping the pager instance
  alive.
- `Flush` finalizes any incomplete parser state after the input stream ends.

Call `Flush` after `ReadFrom` or `ReloadFrom` when the content is complete. It
ensures incomplete escape sequences are resolved into the same safe display
model the rest of the pager uses.

## Sizing And Drawing

`SetSize` should follow the actual viewport the pager occupies, not the whole
terminal by default. If your UI has multiple panes, each pane should size its
own pager instance to the pane rectangle it owns.

After sizing, call `Draw` whenever the content or viewport changes.

```go
width, height := screen.Size()
pager.SetSize(width, height)
pager.Draw(screen)
```

If you want wheel scrolling, enable mouse reporting on the host screen first:

```go
screen.EnableMouse(tcell.MouseButtonEvents)
```

## Handling Input

The pager can handle keys and mouse events directly.

- use `HandleKey` if you only care whether the pager wants to quit
- use `HandleKeyResult` if you also want to inspect whether the event was
  handled plus the action and context
- use `HandleMouse` or `HandleMouseResult` for wheel and pointer events

`HandleKeyResult` is useful in larger applications because it lets the host
decide whether to redraw immediately, update app-level state, or exit.

```go
switch ev := screen.PollEvent().(type) {
case *tcell.EventResize:
	screen.Sync()
	width, height := screen.Size()
	pager.SetSize(width, height)
	pager.Draw(screen)
case *tcell.EventKey:
	result := pager.HandleKeyResult(ev)
	if result.Handled {
		pager.Draw(screen)
	}
	if result.Quit {
		return nil
	}
case *tcell.EventMouse:
	if pager.HandleMouse(ev) {
		pager.Draw(screen)
	}
}
```

## Choosing A Render Mode

The render mode changes how escape sequences are presented.

- `RenderHybrid` is the default fit for most embedders. Supported styling is
  applied, and unsupported sequences remain visible.
- `RenderLiteral` is useful for diagnostics or forensic views where you want
  raw escape text instead of styling.
- `RenderPresentation` applies supported styling and hides unsupported
  sequences when you want a cleaner reading experience.

For hyperlink-rich content, remember that OSC 8 links are still inert unless
your embedder supplies a `HyperlinkHandler`.

## Runtime Reconfiguration

You do not need to rebuild the pager every time your host application changes
state.

- `Configure` accepts runtime-safe options
- `SetTheme`, `SetChrome`, `SetWrapMode`, `SetLineNumbers`, and similar
  setters update a live pager
- `SearchState` exposes the current search information if your host chrome
  wants to mirror it

That makes it practical to keep one pager alive while the surrounding UI swaps
themes, toggles chrome, or changes trust policy for links.
