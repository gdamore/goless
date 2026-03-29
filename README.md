# goless

`goless` is an embeddable pure-Go library for viewing textual content in a
`tcell.Screen` with behavior similar to `less`.

## Status

This repository is in early development.

It is not ready for production use, and the APIs, data structures, and behavior
should all be considered unstable. Expect breaking changes while the design is
still being worked out.

## Goals

- Secure display of untrusted textual input
- ANSI/ECMA-48 style parsing and sanitization
- Unicode grapheme-aware rendering
- Support for wrapping and horizontal scrolling
- Forward and reverse search
- Embeddable APIs for use inside other Go programs

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE).
