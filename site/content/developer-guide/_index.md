+++
title = "Developer Guide"
description = "Guidance for applications that embed goless."
summary = "Practical guidance for applications embedding goless as a pager core."
weight = 20
[params]
  audience = "Embedders"
  focus = "Configuration, integration patterns, security policy, and recipes with code."
+++

The developer guide complements the API reference by focusing on decisions and patterns:

- when to embed `goless`
- how to feed content into the pager
- how to configure rendering, search, chrome, and hyperlink policy
- how to answer common integration questions with code

It should also make the technical fit explicit:

- pure Go implementation
- no cgo dependency in the pager core
- direct rendering to `tcell.Screen`
- a clean path for applications already built on `tcell`

## Who This Is For

This guide is for application authors who want to treat `goless` as a pager
component inside a larger terminal UI rather than as a standalone program.

Typical readers are building one of these:

- a split-pane terminal application
- a log viewer with custom chrome or status text
- a file browser or inspector that wants `less`-style navigation without
  surrendering control of the host terminal
- a tool that needs to display untrusted text safely and predictably

If you only need to run the bundled program, start with the user manual
instead.

## What The Guide Covers

The guide is organized around the integration questions embedders actually hit:

- [Embedding Basics](embedding-basics.md) shows the minimum loop for loading
  content, sizing the pager, and drawing it onto an existing screen.
- [Recipes](recipes.md) answers common customizations with focused examples.
- [Docs Media Plan](media-plan.md) explains how screenshots and casts should
  support the developer-facing docs.

## Mental Model

The pager is a controller around an internal document model. In practice, that
means host applications usually follow the same sequence:

1. Construct a `Pager` once.
2. Load content with `ReadFrom`, `Append`, `AppendString`, or `ReloadFrom`.
3. Call `Flush` when the current input stream is complete.
4. Size the pager to the region it should occupy.
5. Draw it onto the host application's `tcell.Screen`.
6. Forward keyboard, mouse, and resize events back into the pager.
7. Reconfigure rendering or policy at runtime when the host UI changes.

That division keeps the host application in charge of layout and policy while
`goless` focuses on rendering, navigation, search, and safe text handling.

## Security Boundary

`goless` treats the input stream as data rather than as a command channel.

- supported styling is rendered internally instead of being passed through to
  the terminal
- unsupported or malformed sequences are either shown visibly or hidden,
  depending on render mode
- OSC 8 hyperlinks remain inert unless the embedder explicitly opts in
- the pager does not expose shell escape support or subprocess execution

That makes it a good fit for applications that want to display hostile or
arbitrary text without handing terminal control to the input itself. If you
need a concrete starting point, read [Embedding Basics](embedding-basics.md)
next.
