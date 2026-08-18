# Patchflow review artifact v2

Artifact v2 turns every review step into an ordered, agent-authored chapter.
The canonical machine-readable contract is
[`schema/patchflow-review-v2.schema.json`](../schema/patchflow-review-v2.schema.json).
Patchflow's Go validator embeds that exact file and checks the shared fixtures
under `testdata/artifacts/v2`.

## Directory layout

```text
.patchflow/reviews/<review-id>/
  review.yaml
  overview.md
  diagrams/
  assets/
```

`review.yaml` is the source of truth. `overview.md` introduces the change;
ordered blocks in `steps[].blocks` tell its detailed review story. A v2 artifact
does not contain annotations or decisions. The v1 reader remains available for
existing artifacts while interaction design is deferred.

## Chapter structure

Each step retains its `id`, `title`, `priority`, `rationale`, and list of changed
`files`. It also has a non-empty ordered `blocks` list. Block IDs are unique
across the whole review. IDs should describe their subject rather than their
position and remain stable when a block moves between chapters.

Steps may add a `review_question` and an `attention` list. Together they define
what the reviewer should decide and which kind of attention the chapter needs.
A final `takeaway` block records the understanding that closes the chapter while
remaining addressable like every other visible narrative block.

The supported blocks are:

- `prose`: Markdown in `body` that explains the next part of the change.
- `diff`: a changed repository `path`, with optional `view: split | unified`
  and an optional base/target line `focus`. Set `collapsed: true` only for
  supporting, generated, vendored, or repetitive evidence that should not
  consume the first reading pass.
- `code`: a bounded excerpt from `source: base | target`, identified by `path`,
  `start_line`, and `end_line`. Code blocks may reference unchanged files when
  they provide important context.
- `callout`: Markdown `body` with `kind: insight | risk | warning | assumption |
  uncertainty`.
- `question`: a static unresolved question in `body`. Replies are not part of
  v2.
- `diagram`: a Mermaid source `path` below the artifact's `diagrams/` directory.
- `takeaway`: Markdown `body` that closes the chapter with its intended mental
  model. Place it last unless later evidence deliberately follows it.

Example:

```yaml
steps:
  - id: domain
    title: Understand account activation
    priority: high
    rationale: The model owns the state transition.
    review_question: Does the activation rule preserve the intended lifecycle?
    attention: [design, behavior]
    files:
      - app/models/account.rb
    blocks:
      - id: domain-intro
        type: prose
        body: Start with the predicate that guards the transition.
      - id: activation-context
        type: code
        path: app/models/account.rb
        source: target
        start_line: 10
        end_line: 24
      - id: activation-diff
        type: diff
        path: app/models/account.rb
        view: split
        focus:
          side: target
          start_line: 10
          end_line: 24
      - id: activation-risk
        type: callout
        kind: risk
        body: Suspended accounts currently pass the new predicate.
      - id: activation-question
        type: question
        body: Is automatic reactivation intended?
      - id: activation-takeaway
        type: takeaway
        body: The model remains the single owner of account activation.
```

## Semantic invariants

In addition to JSON Schema validation:

- changed paths, step IDs, and block IDs are unique;
- every changed path appears in at least one step;
- step file references and diff blocks reference changed paths;
- code excerpts contain ordered positive line numbers and no more than 500
  lines;
- diagram paths begin with `diagrams/`;
- all paths are normalized and relative, contain no traversal or backslashes,
  and repository paths stay outside `.patchflow`.

Validate with the canonical CLI:

```sh
bin/patchflow validate --format json /path/to/review.yaml
```

The web application exposes `/reviews/<review-id>/blocks/<block-id>` as a stable
reference path. An in-place copy control places that path on the clipboard. If
opened directly, it renders the block's current chapter under the same URL and
focuses the addressed block without a fragment redirect.
