+++
title = "Recipes"
summary = "Common embedder tasks answered with short, focused examples."
weight = 20
+++

This page favors task-oriented answers to "how do I..." questions. Each recipe
should stay narrow enough that a reader can adapt it quickly.

## How Do I Set A Frame Title And Custom Chrome?

Copy a preset, tweak the pieces you need, and pass the result into `WithTheme`
and `WithChrome`.

```go
preset := goless.PrettyPreset
preset.Chrome.Title = "Logs"
preset.Chrome.TitleAlign = goless.TitleAlignLeft

pager := goless.New(
	goless.WithTheme(preset.Theme),
	goless.WithChrome(preset.Chrome),
)
```

Use the preset as a starting point, not as a shared mutable singleton. Copy it
before changing any fields.

Frame glyphs are rendered exactly as configured, so if you want an empty side
or corner, set that field to `""` explicitly.

## How Do I Keep Hyperlinks Inert Until I Trust Them?

Return a decision with `Live: false` by default and opt in only for URLs your
application trusts.

```go
pager := goless.New(
	goless.WithHyperlinkHandler(func(info goless.HyperlinkInfo) goless.HyperlinkDecision {
		if strings.HasPrefix(info.Target, "https://intranet.example.com/") {
			return goless.HyperlinkDecision{
				Live:   true,
				Target: info.Target,
			}
		}

		return goless.HyperlinkDecision{
			Live: false,
		}
	}),
)
```

If you need to rewrite links, set `Target` in the returned decision. If you
need to restyle them, set `StyleSet` and provide a style.

## How Do I Change Search Behavior At Runtime?

Use `Configure` when the pager is already on screen.

```go
pager.Configure(
	goless.WithSearchCaseMode(goless.SearchSmartCase),
	goless.WithSearchMode(goless.SearchWholeWord),
)
```

That is useful when your host UI exposes its own search controls or when you
want to switch between literal and regex matching without rebuilding the pager.

## How Do I Reload Content While Preserving Pager State?

Use `ReloadFrom` on the existing pager and then flush and redraw it.

```go
func reloadFile(pager *goless.Pager, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := pager.ReloadFrom(file); err != nil {
		return err
	}
	pager.Flush()
	return nil
}
```

This keeps the pager instance alive, which makes it easier to preserve chrome,
theme, and host-level integration around the document view.

## How Do I Show Pager State In My Own Status Line?

Use `WithText` and format the left and right status text from `StatusInfo`.

```go
pager := goless.New(
	goless.WithText(goless.Text{
		StatusLine: func(info goless.StatusInfo) (string, string) {
			left := info.Message
			if info.Search.Query != "" {
				left = fmt.Sprintf("%s [%d/%d]", info.Search.Query, info.Search.CurrentMatch, info.Search.MatchCount)
			}

			right := fmt.Sprintf("%d/%d", info.Position.Row, info.Position.Rows)
			return left, right
		},
	}),
)
```

`PromptLine` works the same way if you want to replace the built-in prompt text
instead of only the status bar.

## How Do I Embed Inside A Split-Pane TUI?

Treat each pane as its own viewport. Size the pager to the pane rectangle the
host layout engine assigned it.

```go
func drawPane(screen tcell.Screen, pager *goless.Pager, x, y, width, height int) {
	pager.SetSize(width, height)
	pager.Draw(screen)
}
```

The important part is that the pager never assumes it owns the whole terminal.
The host application stays in charge of layout.

## How Do I Reserve Keys For The Host App?

Use `WithCaptureKey` for keys that the host should consume before pager
handling.

```go
pager := goless.New(
	goless.WithCaptureKey(func(ev *tcell.EventKey) bool {
		return ev.Key() == tcell.KeyCtrlL
	}),
)
```

Use `WithKeyGroup`, `WithUnboundKeys`, and `WithKeyBindings` when you need to
change the pager's own key map.

## Recipe Candidates Worth Adding Later

- custom help overlays
- selective trust policies for multiple views
- command handling for `:` prompts
- logging search state into the host application's own chrome
