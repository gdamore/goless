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

When you are viewing a regular file in the full-screen UI, `v` opens that file
in `$EDITOR` at the current line. Use `-secure` to disable editor launch.

Use `s` to save the current content set or just the visible viewport to a file.
The standalone save command is also disabled by `-secure`.

If you want a default visual theme, `goless` also looks for a per-user JSON
config at `goless/config.json` under the directory returned by
`os.UserConfigDir()`. On macOS that is typically
`~/Library/Application Support/goless/config.json`. On Linux that is typically
`$XDG_CONFIG_HOME/goless/config.json` or `~/.config/goless/config.json`. On
Windows that is typically `%AppData%\goless\config.json`.

Config selection precedence is:

1. `-config path`
2. `GOLESS_CONFIG`
3. the default per-user config path

The initial format is:

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

Use `goless --default-config` to print the built-in JSON and redirect it into a
starter config file.

Command-line flags still win over config values for a single invocation.

Set `"mouse": false` or pass `--no-mouse` if you want the
standalone program to leave terminal text selection and native scrolling alone.

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
