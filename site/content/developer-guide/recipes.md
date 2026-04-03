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
pager := goless.New(goless.Config{
    Chrome: preset.Chrome,
    Theme:  preset.Theme,
})
```

### How do I make hyperlinks explicit instead of automatically live?

```go
pager := goless.New(goless.Config{
    HyperlinkHandler: func(info goless.HyperlinkInfo) goless.HyperlinkDecision {
        return goless.HyperlinkDecision{
            Live: false,
        }
    },
})
```

### How do I switch search behavior?

```go
pager.SetSearchCaseMode(goless.SearchSmartCase)
pager.SetSearchMode(goless.SearchWholeWord)
```

## Future Recipe Backlog

- embed inside a split-pane terminal UI
- implement a custom help overlay or prompt text
- reload content while preserving pager sizing and chrome
- enable trusted links in one view but not another
