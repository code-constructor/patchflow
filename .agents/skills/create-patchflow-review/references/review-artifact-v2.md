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
```

Create `overview.md` beside `review.yaml`. Put Mermaid files below `diagrams/`.
The v2 artifact has no `annotations` or `decisions` fields.

Apply these invariants:

- Use schema version `2` and `overview_path: overview.md`.
- Use IDs containing only letters, digits, `_`, and `-`, beginning with a
  letter or digit. Keep step IDs unique and block IDs unique across the review.
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
  below the artifact's `diagrams/` directory.
- Keep line ranges ordered. Limit code excerpts to 500 lines.
- Use no unknown fields. Set new reviews to `draft`.

The Patchflow application's `bin/patchflow validate --format json` command is
authoritative when available. Fix every error before handing off.
