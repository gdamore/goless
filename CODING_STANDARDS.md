# Coding Standards

## Purpose

This project aims to be a small, secure, embeddable Go library with behavior that is
easy to understand and reliable in production. These standards exist to keep the code,
APIs, and user experience aligned with that goal.

## Core Principles

### Follow Go Idioms

- Adhere to standard Go idioms in naming, package structure, error handling, and API design.
- Prefer simple, explicit code over clever abstractions.
- Use the standard library where it is sufficient.
- Keep packages focused and cohesive.

### Build With Empathy

Empathy is a core engineering value for this project.

We have two classes of users:

- developers embedding this library in their own programs
- end users interacting with programs built on top of it

This means:

- prefer simple, comprehensible interfaces
- keep APIs narrow enough to be understood quickly
- honor the principle of least surprise
- choose behavior that is consistent and easy to explain
- favor debuggability over hidden magic

### Document the Public Surface

- Every exported type, function, method, constant, and variable must have Godoc.
- Documentation should explain behavior, not just restate names.
- Public docs should call out important semantics, limitations, and invariants.
- Public APIs should include examples when that materially improves clarity.

### Design for Testability

- Design code so it can be tested in isolation.
- Prefer dependency injection and small interfaces where they improve testability.
- Include test coverage whenever practical.
- Aim for very high coverage; 100% is a worthwhile target when it remains sensible.
- Treat coverage as a tool, not a substitute for meaningful assertions.

## Additional Project-Specific Standards

### Security First

- Never pass input escape sequences through to the host terminal unchecked.
- Parse and sanitize untrusted input before rendering.
- Avoid features that execute commands or otherwise expand the attack surface.
- Prefer explicit allowlists over implicit acceptance when handling control sequences.

### Correctness Before Optimization

- Prioritize correctness for Unicode, grapheme segmentation, cell width, and escape parsing.
- Do not bake left-to-right display assumptions into core data structures when logical order can be preserved instead.
- Do not introduce performance shortcuts that break rendering or navigation semantics.
- Optimize after measurement, not by guesswork.

### Incremental and Maintainable Design

- Favor designs that support append-only growth and incremental parsing cleanly.
- Keep parsing, document modeling, layout, search, and rendering as separate concerns.
- Avoid data structures that solve problems this project does not actually have.

### Stable and Predictable APIs

- Keep the public API as small as practical.
- Avoid exposing internals prematurely.
- Before initial publication, do not preserve a flawed interface just to avoid churn.
- Pre-1.0, breaking API changes are acceptable when they make the design simpler, clearer, or more correct.
- Once the project has real external users or a declared stable release, tighten compatibility expectations.
- Prefer additive evolution over breaking change after stability becomes a real project constraint.
- Make configuration explicit rather than relying on hidden defaults when behavior is significant.

### Errors Must Be Actionable

- Return errors that help callers understand what failed and where.
- Avoid vague errors when concrete context can be provided.
- Do not hide operationally important failures.

### Testing Expectations

- Unit tests should cover parser behavior, layout behavior, search behavior, and rendering edge cases.
- Unicode edge cases and malformed escape sequences should be treated as first-class test inputs.
- Regressions should be captured with targeted tests before or alongside fixes.
- Fuzzing is encouraged for escape parsing and sanitization paths.

### Keep Comments High Value

- Write comments for exported APIs and non-obvious logic.
- Avoid comments that merely narrate obvious code.
- Prefer making code clearer over explaining unclear code after the fact.

## Review Criteria

Changes should be evaluated against these questions:

1. Is the API obvious and unsurprising to an embedding developer?
2. Is the behavior safe when given hostile or malformed input?
3. Is the Unicode and terminal behavior correct?
4. Is the code easy to test and adequately tested?
5. Is the implementation simpler than the alternatives that were considered?
6. Will future maintainers understand why this works?

## Practical Rule

When in doubt, choose the option that is:

- simpler
- more explicit
- easier to test
- safer for users
- more consistent with normal Go expectations
