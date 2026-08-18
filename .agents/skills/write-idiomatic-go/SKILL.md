---
name: write-idiomatic-go
description: Write, change, refactor, or review clear and maintainable Go code in Patchflow. Use for tasks touching Go source files, go.mod, package boundaries, CLI commands, HTTP handlers, templates, tests, errors, contexts, concurrency, documentation, or Go quality tooling.
---

# Write Idiomatic Go

Produce the smallest clear Go change that fits the repository's architecture,
uses the standard library well, documents its contracts, and is verified with
the project's own toolchain.

## Prepare

1. Read the repository's `AGENTS.md` and `go.mod` before designing the change.
2. Read [the official Go practices](references/official-go-practices.md)
   completely. Treat linked primary sources as the authority when a detail is
   unclear or version-sensitive.
3. Read [the Patchflow conventions](references/patchflow-conventions.md)
   completely for repository-specific architecture and validation rules.
4. Inspect the complete call path, neighboring package APIs, and relevant tests.
   Follow established local conventions unless they conflict with an explicit
   repository rule or hide a correctness problem.

## Design

- Prefer the simplest design that keeps responsibilities and dependencies
  explicit. Do not introduce layers, interfaces, factories, or packages without
  a concrete consumer or boundary.
- Keep executable packages focused on parsing input and wiring dependencies.
  Put reusable behavior in cohesive `internal` packages.
- Define interfaces at the point of use and keep them as small as the consumer
  requires. Accept an interface only when multiple behaviorally meaningful
  implementations, isolation, or substitution justify it.
- Prefer useful zero values and explicit constructors only when invariants or
  dependencies must be established.
- Prefer standard-library facilities. Add a dependency only when its benefit
  exceeds its API, security, binary-size, and maintenance cost.
- Preserve compatibility unless the task explicitly authorizes an API or
  artifact-schema break.

## Implement

- Let `gofmt` decide formatting. Optimize names and control flow for reading,
  not for cleverness or arbitrary size limits.
- Handle errors where a useful decision can be made; otherwise return them with
  concise operational context and preserve their identity with `%w`. Use
  `errors.Is` or `errors.As` when inspecting wrapped errors. Do not both log and
  return the same failure at one layer.
- Reserve `panic` for broken invariants or impossible startup conditions, never
  ordinary user input, filesystem errors, or network failures.
- Pass `context.Context` as the first parameter when work is request-scoped or
  cancelable. Do not store contexts in structs or invent a context merely to
  satisfy an API.
- Make goroutine ownership, cancellation, and completion visible. Never start a
  goroutine without knowing how it stops and who observes its failure.
- Document every named function and method as required by Patchflow. Start Go
  doc comments with the declared name and explain purpose, contract, ownership,
  errors, or side effects rather than translating the function body into prose.
- Keep comments accurate while editing nearby behavior. Delete comments that
  merely narrate syntax.
- Treat request values, repository paths, Git refs, and artifact contents as
  untrusted input. Validate before crossing filesystem or process boundaries.

## Test

- Test observable behavior and boundary conditions, not private implementation
  steps. Add the smallest regression test that would have failed before the
  change.
- Use table-driven tests when cases genuinely share one contract and setup; do
  not turn a single behavior into a ceremonial table.
- Use `t.Helper`, `t.Cleanup`, `t.TempDir`, and `httptest` where they improve
  failure output and isolation. Avoid timing-based sleeps and shared mutable
  global state.
- Check errors and externally visible output precisely enough to protect the
  contract without making tests depend on irrelevant formatting.

## Verify

1. Run the narrowest relevant test while iterating.
2. Run `gofmt` on every changed Go file.
3. Run `bin/ci` before handoff. It checks formatting, `go vet`, all tests, Go
   documentation generation, and a clean build.
4. Run `go test -race ./...` when changing goroutines, synchronization, shared
   state, or request lifetimes.
5. Run `govulncheck ./...` when changing dependencies or security-sensitive
   code if the tool is already available. Do not silently install tools or add
   dependencies merely to complete a check.
6. Report checks that were skipped, unavailable, or failing. Do not disguise an
   environment failure as a passing verification.

## Review the result

Before handing off, inspect the diff for accidental API growth, package cycles,
duplicated error handling, leaked goroutines, stale documentation, unsafe paths,
and tests coupled to implementation details. Explain meaningful design choices
and remaining risks; do not report formatting noise as architecture.
