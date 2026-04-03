+++
title = "Embedding Basics"
summary = "Create a pager, load content, size it, and draw it onto a tcell screen."
weight = 10
+++

This page should answer the first practical integration question: "How do I get a pager on screen?"

```go
pager := goless.New(goless.Config{
    WrapMode:   goless.NoWrap,
    KeyGroup:   goless.LessKeyGroup,
    RenderMode: goless.RenderHybrid,
    ShowStatus: true,
    Chrome: goless.Chrome{
        Title: "Example",
        Frame: goless.RoundedFrame(),
    },
})

_, _ = pager.ReadFrom(strings.NewReader("hello\nworld\n"))
pager.Flush()
pager.SetSize(80, 24)
pager.Draw(screen)
```

## Topics To Cover

- choosing a render mode
- feeding content incrementally versus all at once
- wiring key handling through `HandleKey`
- responding to terminal resize events
- drawing inside a host application without giving terminal control to the content
