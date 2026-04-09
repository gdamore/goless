+++
title = "Navigation and Search"
summary = "Scrolling, paging, following, and searching through content."
weight = 20
+++

The built-in less-like key group already gives the future user manual a strong outline.

## Core Navigation

- `j` and `k` or the arrow keys to move line by line
- `space` and `b` to page down and up
- `w` also pages up in the built-in less-like key group
- `g` and `G` to jump to the top or bottom
- `W` to toggle wrap mode
- `F` to enable follow mode
- `Ctrl-X` or `Ctrl-C` to stop following without quitting

## Search

- `/` starts a forward search
- `?` starts a reverse search
- `n` repeats in the same direction
- `N` repeats in the opposite direction

The final manual should pair each capability with a small terminal capture showing the before-and-after state change.

## Recorded Demo Slot

Store cast files under `site/static/casts/` and embed them with the `asciinema` shortcode:

```md
{{</* asciinema src="/casts/navigation.cast" poster="npt:0:02" rows="28" cols="96" */>}}
```
