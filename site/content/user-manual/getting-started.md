+++
title = "Getting Started"
summary = "Install or build goless, open content, and understand the first screen."
weight = 10
+++

The standalone `goless` program currently lives in this repository under `cmd/goless`:

```bash
go run ./cmd/goless -- file.txt
```

You can also pipe content directly into it:

```bash
printf 'hello\nworld\n' | go run ./cmd/goless
```

If you want to spell stdin explicitly, use `-`:

```bash
printf 'hello\nworld\n' | go run ./cmd/goless -- -
```

When `stdout` is redirected or piped, `goless` falls back to pass-through mode
and writes the selected input unchanged instead of opening the full-screen UI.

## What Users Need First

The first user-facing page should answer these questions quickly:

1. What kinds of files and streams can `goless` open?
2. Which keys matter in the first five minutes?
3. How does the status bar communicate mode and position?
4. What is different from classic `less` on purpose?

## Draft Topics

- launch with a file path
- launch with stdin
- switch among multiple files
- quit behavior and EOF behavior
- wrap mode versus horizontal scrolling

## Screenshot Slot

Store screenshots under `site/static/screenshots/` and reference them with ordinary Markdown:

```md
![Initial view](/screenshots/initial-view.png)
```
