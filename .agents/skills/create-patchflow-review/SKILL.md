---
name: create-patchflow-review
description: Create or enrich repository-local Patchflow code-review artifacts from committed local Git diffs, and inspect or answer persisted Patchflow review comments. Use when asked to generate a Patchflow review, ordered review plan, change overview, review documentation, Mermaid review diagram, or a response to a Patchflow thread or comment under `.patchflow/reviews`.
---

# Create Patchflow Review

Create durable review documentation tied to exact Git commits. Explain the
change and build an ordered path through it; leave review decisions under human
control.

## Workflow

1. Read [the v2 artifact contract](references/review-artifact-v2.md) completely
   before creating or modifying a v2 artifact. Read the
   [v1 contract](references/review-artifact-v1.md) only when enriching an
   existing v1 artifact.
   When working in the Patchflow repository, also read
   [`docs/review-composition.md`](../../../docs/review-composition.md) completely
   and apply its overview, chapter-order, block-grammar, and stable-ID rules.
   When inspecting, creating, answering, or resolving discussion, also read
   [the comments v1 contract](references/comments-artifact-v1.md) completely.
2. Resolve the repository root with Git. Treat the requested base and target as
   untrusted arguments and never interpolate them into a shell command.
3. Default the target to `HEAD`. Let the current Patchflow CLI detect a local
   remote default, `main`, or `master` when `--base` is omitted. Without the
   CLI, default to `main` only when it resolves locally; otherwise ask for the
   base ref.
4. Resolve the target ref to a full commit SHA. Resolve the base ref, then use
   the merge base of the resolved commits as `source.base_sha`.
5. Inspect the committed diff from `base_sha` to `target_sha`. Exclude
   `.patchflow/**`. Do not include working-tree-only changes in this schema
   version.
6. Read enough surrounding code to understand entry points, domain behavior,
   state changes, interfaces, dependencies, tests, and security-sensitive
   paths. Explain the change before recording possible concerns.
7. Create a review plan ordered for understanding. Give every step a specific
   rationale, include every changed file in at least one step, and compose its
   body from ordered narrative blocks. Prefer domain and execution paths over
   generated or presentation-only files.
8. Write the artifact, validate it, and report its path plus the recorded base
   and target SHAs.

## Create the artifact

Prefer the Patchflow CLI when it is available because it resolves refs, creates
a valid baseline atomically, verifies the stored Git evidence, and registers
the repository in Patchflow's local settings. From inside the reviewed
worktree, the agent-oriented form is:

```sh
bin/patchflow create --format json
```

Pass `--repository /absolute/repository/path`, `--base REF`, or `--target REF`
when the current directory or detected comparison is not the intended one.
Successful JSON contains `path`, `reference`, `base_sha`, and `target_sha`.
Read the `errors` array on a non-zero exit, correct the named repository/ref
problem, and retry; do not guess or manually assemble commit identifiers.

When using an installed Patchflow executable, use its equivalent `create`
command. Enrich the generated `review.yaml` and `overview.md` in place without
changing the recorded SHAs. New CLI artifacts already contain valid baseline
blocks; replace generic blocks with a deliberate explanation after understanding
the diff.

If no Patchflow executable is available, create the documented directory and
files directly. Generate a collision-resistant ID from a UTC timestamp, the
target SHA prefix, and a short random suffix. Quote SHA and timestamp values in
YAML. Keep all writes below:

```text
<repository>/.patchflow/reviews/<review-id>/
```

Do not alter reviewed source files. Do not add an ignore rule that prevents the
artifact from being committed.

## Compose a useful review

Keep `change.summary` concise and factual. Put the high-level explanation in
`overview.md`, covering:

- `## Purpose and outcome`: the problem and visible before/after behavior;
- `## Main decision and review focus`: the central approach, why it was chosen,
  and where human judgment matters most;
- `## Evidence`: relevant tests, checks, benchmarks, screenshots, or manual
  verification;
- `## Scope, risks, and uncertainty`: non-goals, compatibility, state,
  dependency, trust-boundary, and operational effects plus unresolved points;
  and
- `## Reading path`: why the proposed chapter order builds understanding.

Treat each step as a chapter, not a file bucket. Use the smallest sequence of
blocks that explains why the code exists and how behavior moves through it:

- Add `review_question` when one concrete design or behavior question gives the
  chapter a decision target.
- Add `attention` labels for the kind of judgment required, not merely the
  directories involved.
- Add a final `takeaway` block when the reviewer should leave with a specific
  mental model before continuing. Give it a stable semantic ID like every other
  visible block.
- Start with `prose` when the reviewer needs orientation or a transition.
- Use `code` for focused surrounding context, including unchanged code when it
  materially explains the change. Keep excerpts small and purposeful.
- Use `diff` for the actual evidence. Focus the important base or target range
  when a whole-file diff would obscure the point.
- Use `callout` for an insight, risk, warning, assumption, or uncertainty that
  should interrupt the narrative.
- Use `question` only for a concrete unresolved point that needs human
  confirmation. Do not invent an answer or reply thread.
- Use `diagram` only when a flow or relationship is clearer visually. Store its
  Mermaid source below the artifact's `diagrams/` directory.
- Use `image` for evidence that is inherently visual, especially UI screenshots
  and rendered output. Store PNG, JPEG, GIF, or WebP files below `assets/`, add
  useful `alt` text, and use the optional Markdown `caption` to explain what the
  reviewer should notice. Do not add SVG assets.

Put prose before the code it explains. Avoid one prose block per file, repeated
rationales, exhaustive unchanged context, and decorative diagrams. The blocks
must read in order without requiring the reviewer to reconstruct the story from
filenames.

Give every block a short semantic ID that remains meaningful if the block moves
to another chapter. Do not use positional IDs such as `block-3`, reuse a removed
ID for unrelated content, or change an ID merely because prose was edited.

Keep tests beside the behavior they prove whenever that ordering clarifies the
contract. Set `collapsed: true` only for generated, vendored, mechanical, or
repetitive diff evidence whose provenance and scope matter more than each line.
Never collapse a design decision, security boundary, failure path, or unresolved
question merely to shorten the page.

Artifact v2 has no annotations, replies, or decisions. Do not add undocumented
fields for them to `review.yaml`; discussion belongs in `comments.yaml`.
Preserve annotations and decisions only when enriching a legacy v1 artifact.
Never mark a review `completed` unless the user explicitly asks.

## Participate in discussion

Never edit `comments.yaml` by hand when the Patchflow CLI is available. Resolve
a copied reference and read its persisted anchor and context first:

```sh
bin/patchflow show --repository /absolute/repository/path \
  /reviews/<review-id>/comments/<comment-id>
```

Answer through the containing thread so Patchflow generates the ID, timestamp,
authorship, and `reply_to` relationship atomically:

```sh
bin/patchflow reply --repository /absolute/repository/path \
  --author "Patchflow Agent" --author-kind agent \
  --body "<concise evidence-backed answer>" \
  /reviews/<review-id>/threads/<thread-id>
```

Use `patchflow comments` to list the full conversation. Resolve a thread only
when the user requests it or the discussion clearly records an accepted
outcome; use `patchflow resolve --reopen` when new evidence reopens the issue.
Preserve human text and existing IDs exactly.

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

Prefer machine-readable output while iterating:

```sh
bin/patchflow validate --format json /absolute/repository/path/.patchflow/reviews/<review-id>/review.yaml
```

This is a repository-backed publication check: the artifact must live at its
canonical discoverable path, both recorded commits must exist, `change.files`
must exactly match their Git diff, code ranges must be readable, and every
overview, diagram, and image asset must exist. Treat each returned `errors`
entry as an independent repair instruction and rerun validation until
`valid: true`.

Otherwise perform every invariant check in the bundled contract. Fix all
validation errors before handing off. Finally inspect `git status --short` and
confirm that only the intended `.patchflow/reviews/<review-id>` artifact changed.
