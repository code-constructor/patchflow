---
name: create-patchflow-review
description: Create or enrich repository-local Patchflow code-review artifacts from committed local Git diffs. Use when asked to generate a Patchflow review, ordered review plan, change overview, review documentation, or Mermaid review diagram under `.patchflow/reviews`, and when updating or validating an existing Patchflow artifact against base and target commits.
---

# Create Patchflow Review

Create durable review documentation tied to exact Git commits. Explain the
change and build an ordered path through it; leave review decisions under human
control.

## Workflow

1. Read [the v1 artifact contract](references/review-artifact-v1.md) completely
   before creating or modifying an artifact.
2. Resolve the repository root with Git. Treat the requested base and target as
   untrusted arguments and never interpolate them into a shell command.
3. Default the target to `HEAD`. Default the base to `main` only when `main`
   resolves locally; otherwise ask for the base ref.
4. Resolve the target ref to a full commit SHA. Resolve the base ref, then use
   the merge base of the resolved commits as `source.base_sha`.
5. Inspect the committed diff from `base_sha` to `target_sha`. Exclude
   `.patchflow/**`. Do not include working-tree-only changes in this schema
   version.
6. Read enough surrounding code to understand entry points, domain behavior,
   state changes, interfaces, dependencies, tests, and security-sensitive
   paths. Explain the change before recording possible concerns.
7. Create a review plan ordered for understanding. Give every step a specific
   rationale and include every changed file in at least one step. Prefer domain
   and execution paths over generated or presentation-only files.
8. Write the artifact, validate it, and report its path plus the recorded base
   and target SHAs.

## Create the artifact

Prefer the Patchflow CLI when it is available because it resolves refs and
creates a valid baseline atomically:

```sh
bin/patchflow create --repository /absolute/repository/path --base main --target HEAD
```

When using an installed Patchflow executable, use its equivalent `create`
command. Enrich the generated `review.yaml` and `overview.md` in place without
changing the recorded SHAs.

If no Patchflow executable is available, create the documented directory and
files directly. Generate a collision-resistant ID from a UTC timestamp, the
target SHA prefix, and a short random suffix. Quote SHA and timestamp values in
YAML. Keep all writes below:

```text
<repository>/.patchflow/reviews/<review-id>/
```

Do not alter reviewed source files. Do not add an ignore rule that prevents the
artifact from being committed.

## Write useful documentation

Keep `change.summary` concise and factual. Put the fuller explanation in
`overview.md`, covering:

- the purpose and visible behavior of the change;
- the important execution or data flow;
- state, interface, dependency, and trust-boundary changes;
- why the proposed review order builds understanding; and
- uncertainties that require reviewer confirmation.

Add Mermaid only when a flow or relationship is clearer as a diagram. Preserve
the Mermaid source in `overview.md` or the artifact's `diagrams/` directory.

Use annotations for observations tied to the overview, a file, or exact base or
target lines. Preserve all existing annotations and decisions when enriching an
artifact. Never mark a review `completed` unless the user explicitly asks.

## Handle existing and stale reviews

Load and validate an existing artifact before editing it. Compare its
`source.target_sha` with the currently resolved target ref.

- If they match, enrich the artifact without replacing human-authored content.
- If they differ, preserve the existing artifact as evidence and create a new
  review unless the user explicitly requests a revision.
- Never silently rewrite `base_sha` or `target_sha` in an existing artifact.

## Validate

Run the canonical validator when Patchflow is available:

```sh
bin/patchflow validate /absolute/repository/path/.patchflow/reviews/<review-id>/review.yaml
```

Otherwise perform every invariant check in the bundled contract. Fix all
validation errors before handing off. Finally inspect `git status --short` and
confirm that only the intended `.patchflow/reviews/<review-id>` artifact changed.
