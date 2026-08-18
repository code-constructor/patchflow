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
  be versioned in Git. SQLite is the only persistence layer Patchflow itself
  needs at this stage.
- Agent-guided, human-controlled: agents propose review order, explanations,
  and annotations; the human remains able to inspect, amend, and decide.
- Explain before critique: prioritize model/domain logic, security-sensitive
  flows, interfaces, state changes, and dependencies over presentation-only or
  generated files. Every priority needs a visible rationale.
- Preserve evidence: artifacts must record the exact base and target commit
  SHAs they describe. A changed target is stale and requires a new or revised
  review artifact.

## Initial Product Scope

Build a vertical slice before recreating a full GitHub-like diff viewer:

1. Select a local Git diff and resolve its base and target SHAs.
2. Create and validate a machine-readable review artifact.
3. Render a review overview with an ordered set of review steps.
4. Show a simple unified text diff in the planned order.
5. Support overview, file, and line/range annotations.
6. Render Markdown and Mermaid diagrams in review documentation.

Split diffs, large-diff virtualization, GitHub PR import, notebook/image
viewers, and elaborate keyboard navigation are later capabilities, not
prerequisites for the first usable version.

## Artifact Direction

Use a repository-local convention such as:

```text
.patchflow/reviews/<review-id>/
  review.yaml
  overview.md
  diagrams/
  assets/
```

`review.yaml` is the machine-readable source of truth. It should capture a
schema version, immutable source refs, change summary, ordered review steps,
priority rationale, annotations, decisions, and review status. Mermaid source
belongs in the artifact directory rather than existing only as rendered output.

Patchflow must avoid reviewing its own generated review artifact by default.

## Technical Direction

- Rails 8.1, Ruby 3.4, SQLite.
- Use Rails conventions, Hotwire/Turbo, Stimulus, Importmap, and server-rendered
  HTML by default.
- Prefer Rails and browser-native capabilities over adding a JavaScript framework.
- The Rails defaults for Solid Cache, Solid Queue, and Solid Cable are acceptable
  because they remain local and SQLite-backed. Do not require Redis, PostgreSQL,
  Docker, or a cloud service for the initial product.
- Use small service objects for Git access, artifact parsing, and plan generation;
  do not let controllers own Git commands or filesystem policy.
- Treat all filesystem paths from requests or artifacts as untrusted. Resolve
  them relative to the selected repository and reject traversal outside it.

## Working Agreement

- Keep the application runnable with `bin/rails` and SQLite alone.
- Add focused tests for new behavior. Run `bin/rails test` before handoff.
- Do not commit `vendor/bundle`, SQLite database files, logs, temporary files,
  or machine-specific configuration.
- Make product decisions and artifact-schema changes visible in committed docs.
