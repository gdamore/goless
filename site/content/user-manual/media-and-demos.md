+++
title = "Media and Demos"
summary = "Where screenshots and terminal recordings fit into the user docs."
weight = 30
+++

This page keeps screenshots and terminal recordings in scope while the user
manual is still being filled out.

## Asset Locations

- screenshots: `site/static/screenshots/`
- terminal casts: `site/static/casts/`
- logos and brand assets: `site/static/assets/logos/`

## Capture Checklist

- keep terminal dimensions stable across recordings
- use sample content that highlights styling, search, and wrapping
- avoid personal paths, hostnames, or machine-specific prompts
- capture both light and dark theme examples if the UI diverges visually

## Planned Demo Topics

- opening a file and moving around quickly
- searching, repeating, and clearing search
- toggling wrap and hidden-character visualization
- moving across multiple files in one session

## WEKActl Demo

This recording shows `goless` running inside a third-party terminal program,
`wekactl`, so readers can see how the pager behaves when embedded in another
application instead of launched on its own.

{{< iframe src="/demos/wc.html" title="goless running inside wekactl" height="520" >}}

Keep the filename stable so the documentation can point at it directly from
other guide pages.
