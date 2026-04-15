+++
title = "Docs Media Plan"
summary = "How screenshots and recorded demos support the developer-facing docs."
weight = 30
+++

Developer docs benefit from media too, but the goal is different from the user
manual. Here the visuals should help embedders understand API choices, chrome,
and behavior changes.

## Best Uses For Visuals

- screenshots that label chrome, status, and layout features
- short `.cast` recordings that show integration behavior, not just the pager
  alone
- paired code and UI examples so embedders can connect configuration to
  outcomes

## What Each Asset Should Show

- the first screenshot should explain the default embedding shape quickly
- a second screenshot can show a changed option, such as a different frame,
  search mode, or link policy
- a cast should demonstrate one interaction thread from start to finish, such
  as loading content, searching, and reloading
- if a visual mentions hyperlink policy, the surrounding text should explain
  why the link is live or inert

## Authoring Notes

- prefer narrow, task-specific recordings over one long demo
- capture examples that show API choices changing visible behavior
- keep image and cast file names stable so content links stay clean
- use the same viewport size for related screenshots unless the size itself is
  the point
- avoid recording noisy terminal setup when the doc needs the pager behavior
  itself

## Suggested File Placement

- store screenshots under `site/static/screenshots/`
- store casts under `site/static/casts/`
- keep any adjacent explanatory text in the same page so the media does not
  float without context

## Practical Check

Before publishing a media update, verify that the visual answers one of these
questions:

- what option changed
- what effect did that option have
- what should the embedder do differently because of that effect
