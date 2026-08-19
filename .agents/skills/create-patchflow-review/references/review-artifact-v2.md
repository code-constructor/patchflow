# Patchflow review artifact v2

Create `.patchflow/reviews/<review-id>/review.yaml` with this structure. The
canonical JSON Schema is `schema/patchflow-review-v2.schema.json` in the
Patchflow repository.

```yaml
schema_version: 2
id: <review-id>
repository:
  name: <display-name>
source:
  base_ref: <requested base ref>
  target_ref: <requested target ref>
  base_sha: <full diff-base commit SHA>
  target_sha: <full target commit SHA>
created_at: <ISO 8601 UTC timestamp>
updated_at: <ISO 8601 UTC timestamp>
status: draft | in_review | completed | stale
change:
  title: <short title>
  summary: <plain-text summary>
  files:
    - path: <repository-relative path>
      status: added | modified | deleted | renamed | copied | type_changed | unmerged | unknown
      previous_path: <required only for renamed or copied files>
overview_path: overview.md
steps:
  - id: <step-id>
    title: <short chapter title>
    priority: critical | high | medium | low
    rationale: <why this chapter belongs at this position>
    review_question: <optional decision or behavior the reviewer should verify>
    attention: # optional, unique values
      - design | behavior | data | security | reliability | performance | ux | operations | verification | documentation | mechanical
    files:
      - <changed path>
    blocks:
      - id: <globally unique block-id>
        type: prose
        body: <Markdown explanation>
      - id: <globally unique block-id>
        type: diff
        path: <changed path>
        view: split | unified # optional
        collapsed: true | false # optional; use true for supporting or noisy evidence
        focus: # optional
          side: base | target
          start_line: <positive integer>
          end_line: <optional integer not before start_line>
      - id: <globally unique block-id>
        type: code
        path: <repository-relative path; may be unchanged context>
        source: base | target
        start_line: <positive integer>
        end_line: <integer at or after start_line, at most 500 lines>
      - id: <globally unique block-id>
        type: callout
        kind: insight | risk | warning | assumption | uncertainty
        body: <Markdown explanation>
      - id: <globally unique block-id>
        type: question
        body: <concrete unresolved question>
      - id: <globally unique block-id>
        type: diagram
        path: diagrams/<name>.mmd
      - id: <globally unique block-id>
        type: image
        path: assets/<name>.png # PNG, JPEG, GIF, or WebP
        alt: <accessible description of the image>
        caption: <optional Markdown explaining what to inspect>
      - id: <globally unique block-id>
        type: takeaway
        body: <understanding the reviewer should leave with>
```

Create `overview.md` beside `review.yaml`. Put Mermaid files below `diagrams/`
and raster review images below `assets/`.
The v2 narrative has no `annotations`, replies, or `decisions` fields. Persisted
discussion uses the separate `comments.yaml` contract; never add its fields to
`review.yaml`.

Apply these invariants:

- Use schema version `2` and `overview_path: overview.md`.
- Use IDs containing only letters, digits, `_`, and `-`, beginning with a
  letter or digit. Keep step IDs unique and block IDs unique across the review.
  Prefer semantic block IDs that remain useful when a block moves; do not encode
  only an array position.
- Quote full lowercase 40-character Git SHAs and ISO 8601 UTC timestamps.
- Record the actual merge base in `base_sha`, not merely the current base-ref
  tip. Treat both recorded SHAs as immutable.
- Include at least one changed file, one review step, and one block per step.
  Use each changed path once in `change.files`; reference only changed paths
  from step file lists and diff blocks; cover every changed path in at least one
  step.
- Code blocks may reference unchanged files when necessary for context.
- Use normalized relative paths with `/` separators. Reject absolute paths,
  `..`, backslashes, and repository paths below `.patchflow`. Keep diagram paths
  below the artifact's `diagrams/` directory and image paths below `assets/`.
- Images require useful `alt` text and must be PNG, JPEG, GIF, or WebP. Do not
  use SVG for untrusted review assets.
- Keep line ranges ordered. Limit code excerpts to 500 lines.
- Use no unknown fields. Set new reviews to `draft`.
- Use `review_question` and `attention` when they sharpen a chapter's purpose.
  Use a final `takeaway` block when a specific mental model should close the
  chapter. Collapse only evidence whose initial expansion would spend attention
  without improving the reviewer's first-pass understanding.

The Patchflow application's `bin/patchflow validate --format json` command is
authoritative when available. Fix every error before handing off.
