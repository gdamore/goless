+++
title = "Recipes"
summary = "Common embedder tasks answered with short, focused examples."
weight = 20
+++

The developer guide should favor recipe-style answers to "how do I..." questions.

## Example Recipe Areas

### How do I set a frame title and custom chrome?

```go
preset := goless.PrettyPreset
preset.Chrome.Title = "Logs"
pager := goless.New(
    goless.WithChrome(preset.Chrome),
    goless.WithTheme(preset.Theme),
)
```

### How do I make hyperlinks explicit instead of automatically live?

```go
pager := goless.New(
    goless.WithHyperlinkHandler(func(info goless.HyperlinkInfo) goless.HyperlinkDecision {
        return goless.HyperlinkDecision{
            Live: false,
        }
    }),
)
```

### How do I switch search behavior?

```go
pager.Configure(
    goless.WithSearchCaseMode(goless.SearchSmartCase),
    goless.WithSearchMode(goless.SearchWholeWord),
)
```

### How do I reload content while preserving pager state?

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

## Future Recipe Backlog

- embed inside a split-pane terminal UI
- implement a custom help overlay or prompt text
- enable trusted links in one view but not another
