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

Use wrap mode when long lines matter more than preserving exact horizontal
position. Leave it off when you are comparing aligned text or reading tables.
- `j` and `k` to move line by line
- `Up` and `Down` to move by a coarse vertical screen step
- `Shift-Up` and `Shift-Down` to move line by line with the cursor keys
- `space` and `b` to page down and up
- `w` also pages up in the built-in less-like key group
- `g` and `G` to jump to the top or bottom
- `W` to toggle wrap mode
- `F` to enable follow mode
- `Ctrl-X` or `Ctrl-C` to stop following without quitting
- `Ctrl-Z` to suspend and resume the terminal session

## Search

- `/` starts a forward search.
- `?` starts a reverse search.
- `n` repeats in the same direction.
- `N` repeats in the opposite direction.

Search should behave like a terminal pager, not a text editor: predictable,
directional, and easy to repeat without leaving the screen.

## Recording Slot

This page is also where the first terminal demo should live once you have a
recording to drop in.

{{< asciinema poster="npt:0:02" rows="28" cols="96" >}}

If the cast file is not ready yet, the shortcode will show a placeholder so the
page still makes sense during authoring.
