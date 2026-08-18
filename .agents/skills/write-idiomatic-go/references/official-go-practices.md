# Official Go practices

Use these principles as defaults, not as substitutes for reading the code in
front of you. When a question is version-sensitive, follow the linked primary
source for the Go version declared by the module.

## Source hierarchy

1. The language specification and standard-library package documentation define
   behavior.
2. Current Go documentation defines supported project and tooling practices.
3. [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) collects
   common review guidance but explicitly does not replace judgment.
4. [Effective Go](https://go.dev/doc/effective_go) remains useful for core
   idioms, but its own introduction says it has not been updated substantially
   and does not cover the modern ecosystem, modules, testing, or newer language
   features completely. Never treat it as a frozen style law.

## Formatting and names

- Run `gofmt`; do not manually align or debate formatting it owns.
- Choose short names whose meaning is clear in their scope. Avoid type-stuttering
  such as `http.HTTPServer` and package names such as `util`, `common`, or
  `helpers` that conceal responsibility.
- Use initialisms consistently (`ID`, `HTTP`, `URL`). Name getters `Owner`, not
  `GetOwner`, unless `Get` is part of the domain operation.
- Keep the happy path visible with early returns for errors and exceptional
  cases. Avoid an `else` after a branch that returns.

Primary guidance: [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
and [Effective Go](https://go.dev/doc/effective_go).

## Packages and APIs

- Organize packages around cohesive behavior. Start small; split when a package
  has distinct responsibilities or dependency pressure, not to imitate a
  generic architecture diagram.
- Use `internal` to enforce repository-private APIs. A `cmd/<name>` directory is
  appropriate for an executable entry point; keep business logic out of it.
- Avoid a `pkg` directory unless the repository intentionally publishes packages
  for external consumers.
- Keep exported APIs minimal. Return concrete types by default and define small
  interfaces where they are consumed.
- Make the zero value useful when practical. Use constructors to establish
  required invariants, not automatically for every type.

Primary guidance: [Organizing a Go module](https://go.dev/doc/modules/layout)
and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).

## Documentation

- Write a package comment for every package. Begin an exported declaration's
  doc comment with its declared name so documentation reads naturally and tools
  associate it correctly.
- Document semantics that callers need: ownership, units, concurrency safety,
  mutation, valid values, returned errors, and side effects.
- Prefer complete sentences and runnable examples for APIs whose intended use
  is not obvious.

Primary guidance: [Go Doc Comments](https://go.dev/doc/comment).

## Errors and cleanup

- Return errors as values. Add useful context as an error crosses an abstraction
  boundary and use `%w` when callers may need the underlying identity.
- Inspect chains with `errors.Is` and `errors.As`, not string matching.
- Keep error strings lowercase and without trailing punctuation when they will
  be wrapped into a larger message.
- Do not use `panic` for expected failures. Pair every acquired resource with a
  clear cleanup path, commonly a nearby `defer` after successful acquisition.

Primary guidance: [`errors`](https://pkg.go.dev/errors),
[`fmt.Errorf`](https://pkg.go.dev/fmt#Errorf), and
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).

## Context and concurrency

- Pass a context explicitly as the first parameter. Do not store it in a struct,
  pass `nil`, or use it for optional arguments.
- Propagate cancellation and deadlines through request-scoped calls.
- Keep synchronization at the owner of the protected state. Document whether
  exported values are safe for concurrent use.
- Give every goroutine a termination condition. Prefer synchronous code when
  concurrency does not provide a concrete benefit.

Primary guidance: [`context`](https://pkg.go.dev/context) and
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).

## Tests and tools

- Put tests beside the package they exercise. Use external `_test` packages only
  when testing the public API boundary is intentional.
- Prefer deterministic tests with useful failure messages. Use subtests and
  table-driven tests when they make a family of cases easier to read.
- Use `go test`, `go vet`, and `gofmt` as baseline checks. Use the race detector
  for concurrency-sensitive changes and fuzzing for parsers or boundary-heavy
  inputs when it is likely to expose defects.

Primary guidance: [`testing`](https://pkg.go.dev/testing),
[`go vet`](https://pkg.go.dev/cmd/vet),
[the race detector](https://go.dev/doc/articles/race_detector), and
[fuzzing](https://go.dev/doc/security/fuzz/).
