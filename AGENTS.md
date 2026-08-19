# Patchflow

## Product Brief

Patchflow is a local-first, agent-guided code-review application. It replaces the
default "read every changed file in Git order" workflow with an explicit review
plan: an agent first explains the change, identifies the important execution and
domain paths, and guides a reviewer through them in the order that best builds
understanding.

The output of a review is documentation, not ephemeral browser state. Plans,
decisions, annotations, diagrams, and findings must be stored as review
artifacts inside the reviewed repository so they can be committed with the
change and understood later.

## Core Principles

- Local first: Patchflow runs on the developer's machine. Do not introduce a
  hosted service, account system, or network dependency without an explicit
  product decision.
- Git-native: review artifacts belong in the target project and are intended to
  be versioned in Git. Patchflow needs no application database at this stage.
- Agent-guided, human-controlled: agents propose review order, explanations,
  and annotations; the human remains able to inspect, amend, and decide.
- Explain before critique: prioritize model/domain logic, security-sensitive
  flows, interfaces, state changes, and dependencies over presentation-only or
  generated files. Every priority needs a visible rationale.
- Preserve evidence: artifacts must record the exact base and target commit
  SHAs they describe. A changed target is stale and requires a new or revised
  review artifact.
- Addressable by default: every narrative block has a stable semantic ID.
  Resource identity belongs in URL paths, shareable presentation state in query
  parameters, and an addressed block directly in its own resource path. Do not
  hide shareable UI state in application storage.

## Initial Product Scope

Build a vertical slice before recreating a full GitHub-like diff viewer:

1. Select a local Git diff and resolve its base and target SHAs.
2. Create and validate a machine-readable review artifact.
3. Render a review overview with an ordered set of review steps.
4. Show a simple unified text diff in the planned order.
5. Compose chapters from prose, code, diff, callout, question, diagram, image,
   and takeaway blocks.
6. Render Markdown, Mermaid, syntax-highlighted code, and split/unified diffs.
7. Discuss whole blocks or immutable source ranges through addressable threads
   shared by the browser, CLI, and Coding Agents.
8. Switch between the guided plan and an addressable changed-file tree without
   losing the reviewer's place in either view.

Large-diff virtualization beyond viewport loading, GitHub PR import, notebook viewers, and
elaborate keyboard navigation are later capabilities.

## Artifact Direction

Use a repository-local convention such as:

```text
.patchflow/reviews/<review-id>/
  review.yaml
  overview.md
  diagrams/
  assets/
```

`review.yaml` is the machine-readable source of truth for the review narrative. It should capture a
schema version, immutable source refs, change summary, ordered review steps,
priority rationale, narrative blocks, and review status. Mermaid source belongs
under `diagrams/`; PNG, JPEG, GIF, and WebP review images belong under `assets/`. Legacy
v1 annotations and decisions remain readable; v2 deliberately omits them.
Mutable discussion lives in a separately versioned `comments.yaml` beside the
review so agent-authored narrative and reviewer conversation remain independent.

Patchflow must avoid reviewing its own generated review artifact by default.

## Technical Direction

- Go 1.24+ with the standard library HTTP server and `html/template`.
- Use server-rendered HTML, Turbo, Stimulus, native ES modules, an Import Map,
  plain CSS, and `go:embed`. Do not add Node, Vite, or a frontend build pipeline.
- Prefer small packages for Git access, artifact parsing, persistence, plan
  generation, and HTTP rendering; handlers do not own Git commands or
  filesystem policy.
- Ship the application as one self-contained binary with embedded templates and
  browser assets. Do not require a database, Redis, Docker, or a cloud service.
- Treat all filesystem paths from requests or artifacts as untrusted. Resolve
  them relative to the selected repository and reject traversal outside it.
- Keep semantic review content in Git-native artifacts. Use URLs for navigation,
  focus, and presentation state; never encode review prose or absolute local
  paths into shareable URLs.
- Keep visual design values behind the semantic CSS tokens in `:root`. Add or
  override palette, typography, surface, border, radius, shadow, and layout
  tokens instead of scattering literal theme values across components.

## Working Agreement

- Keep the application runnable with `bin/patchflow` and Git alone.
- Document every named function and method, including unexported helpers and
  tests. Go Doc comments start with the declared name and explain purpose,
  contract, or important side effects instead of restating the implementation.
  Browser-controller methods follow the same rule with concise JSDoc comments.
- Keep `bin/docs` working so the complete Go package documentation can always
  be rendered from source with standard Go tooling.
- Add focused tests for new behavior. Run `bin/ci` before handoff.
- Do not commit compiled binaries, temporary files, or machine-specific
  configuration.
- Make product decisions and artifact-schema changes visible in committed docs.
- Follow `docs/review-composition.md` when generating or changing the narrative
  structure of a review.
