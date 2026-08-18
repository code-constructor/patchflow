# Patchflow Go conventions

These repository rules intentionally narrow the broader set of valid Go
designs. `AGENTS.md` remains authoritative if this reference becomes stale.

## Product and runtime boundary

- Target Go 1.24 or newer as declared by `go.mod`.
- Ship one local, self-contained binary. Do not introduce a hosted service,
  database, Redis, Node runtime, Vite, or another frontend build pipeline.
- Use the standard-library HTTP server and `html/template`. Serve Turbo,
  Stimulus, native ES modules, the import map, CSS, templates, and other browser
  assets from the embedded binary.
- Keep `cmd/patchflow` as the composition root. Put Git access, artifacts,
  review behavior, and HTTP rendering in focused `internal` packages.
- Prefer the standard library. Existing focused dependencies are acceptable;
  new dependencies require a concrete advantage and explicit maintenance cost.

## Repository boundaries

- Treat local paths, Git refs, URL parameters, review YAML, referenced assets,
  and line ranges as untrusted.
- Resolve repository-owned paths against a canonical repository root and reject
  traversal or symlink escapes before reading or writing.
- Execute Git with separated arguments. Never build a shell command from a ref,
  path, or request value.
- Preserve exact base and target commit SHAs in review artifacts. Exclude
  `.patchflow/**` from the reviewed diff.
- Keep handlers focused on HTTP concerns; they must delegate Git commands,
  filesystem policy, validation, and review construction to the owning package.

## Documentation contract

- Document every named Go function and method, including unexported helpers and
  test functions. This is deliberately stricter than standard `golint`-style
  exported-API documentation.
- Start every function or method comment with its declared name. Explain why it
  exists, its contract, or an important side effect; do not paraphrase each line
  of its body.
- Keep a package comment in `doc.go` or an appropriate package file.
- Keep `bin/docs` working. It renders all package documentation with the standard
  `go doc` tool and includes unexported declarations so the repository-wide
  documentation rule is enforceable.

## Tests

- Co-locate focused tests with their owning package.
- Prefer temporary Git repositories and `t.TempDir` for repository behavior.
- Use `httptest` for routes and rendered responses. Assert both status and the
  smallest meaningful response fragment.
- Protect filesystem escape checks, stale-artifact behavior, schema validation,
  and diff rendering with explicit negative cases.
- Do not make tests depend on the developer's repository, Docker mount, current
  branch, global Git configuration, or network access.

## Required handoff checks

Run focused tests during implementation, then run:

```sh
bin/ci
```

`bin/ci` verifies:

- `gofmt` for `schema`, `internal`, and `cmd`;
- `go vet ./...`;
- `go test ./...`;
- full documentation generation through `bin/docs`; and
- a trimmed build of `cmd/patchflow`.

Additionally run `go test -race ./...` after changing goroutines,
synchronization, shared state, or request cancellation. Never claim these checks
passed without reading their exit status.
