+++
title = "Navigation and Search"
summary = "Scrolling, paging, following, and searching through content."
weight = 20
+++

This page covers the key interactions you will use most often once a document
is open.

## Movement

- `j` and `k` move one line at a time.
- `Up` and `Down` move by a coarse vertical screen step.
- `Shift-Up` and `Shift-Down` move line by line with the cursor keys.
- `space` and `b` page down and up.
- `w` also pages up in the built-in less-like key group.
- `g` and `G` jump to the top or bottom.
- `W` toggles wrap mode.
- `F` enables follow mode.
- `Ctrl-X` or `Ctrl-C` stop following without quitting.
- `Ctrl-Z` suspends and resumes the terminal session.

Use wrap mode when long lines matter more than preserving exact horizontal
position. Leave it off when you are comparing aligned text or reading tables.

## Search

- `/` starts a forward search.
- `?` starts a reverse search.
- `n` repeats in the same direction.
- `N` repeats in the opposite direction.

Search should behave like a terminal pager, not a text editor: predictable,
directional, and easy to repeat without leaving the screen.

## Commands

- `q` / `Esc` quit.
- `r` / `Ctrl-L` repaint the screen.
- `F1` shows the built-in help overlay.
- `F4` cycles visual themes.
- `F5` toggles hidden-character markers.
- `s` opens the standalone save prompt.
- `v` opens the current file in `$EDITOR` at the current line.
- `:number` / `:nonumber` show or hide line numbers.
- `:wrap` / `:nowrap` toggle soft wrapping.
- `:markers` / `:nomarkers` show or hide hidden-character markers.
- `:squeeze` / `:nosqueeze` collapse or keep adjacent blank lines.
- `:tabs <n>` sets tab width.
- `:pin [rows=<n>] [cols=<n>]` pins the top rows and/or left columns.
- `:match [auto|nocase|case] [sub|word|regex]` controls search case and matching.
- `:reload` reloads the current file or input.
- `:save`, `:write`, and `:w` save the current content set or the visible viewport.
- `:next` / `:prev` move through a multi-file session.
- `:first`, `:last`, `:x`, and `:file` control which file is active.
- `=` or `Ctrl-G` show the current file or session status.
- `:license` shows the bundled Apache license.
- `:Q` quits.
- `:help` shows the built-in help overlay.

The standalone save prompt uses the same save command. `F2` toggles file versus
viewport scope, and `F3` toggles ANSI versus plain output while you are editing
the prompt.

## Recording Slot

This page is also where the first terminal demo should live once you have a
recording to drop in.

{{< asciinema poster="npt:0:02" rows="28" cols="96" >}}

If the cast file is not ready yet, the shortcode will show a placeholder so the
page still makes sense during authoring.
