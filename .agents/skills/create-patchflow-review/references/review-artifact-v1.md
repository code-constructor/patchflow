# Patchflow review artifact v1

Create `.patchflow/reviews/<review-id>/review.yaml` with exactly these fields:

```yaml
schema_version: 1
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
    title: <short title>
    priority: critical | high | medium | low
    rationale: <why this step belongs at this position>
    files:
      - <path present in change.files>
annotations:
  - id: <annotation-id>
    scope: overview | file | line
    body: <Markdown text>
    file_path: <required for file and line scopes>
    side: base | target # required for line scope
    start_line: <positive integer> # required for line scope
    end_line: <positive integer not before start_line> # optional
    created_at: <ISO 8601 UTC timestamp>
decisions:
  - id: <decision-id>
    summary: <decision text>
    status: open | accepted | rejected
    rationale: <optional Markdown text>
    created_at: <ISO 8601 UTC timestamp>
```

Create `overview.md` beside `review.yaml`. Optional `diagrams/` and `assets/`
directories also belong beside it.

Apply these invariants:

- Use schema version `1` and `overview_path: overview.md`.
- Use IDs containing only letters, digits, `_`, and `-`, beginning with a
  letter or digit. Keep IDs unique within their collection.
- Quote full lowercase 40-character Git SHAs and ISO 8601 UTC timestamps.
- Record the actual merge base in `base_sha`, not merely the current base-ref
  tip. Treat both recorded SHAs as immutable.
- Include at least one changed file and one review step. Use each changed path
  once in `change.files`; reference only changed paths from steps and
  annotations; cover every changed path in at least one step.
- Use normalized repository-relative paths with `/` separators. Reject absolute
  paths, `..`, backslashes, and all `.patchflow` paths.
- Require `file_path` for file annotations. Require `file_path`, `side`, and a
  positive `start_line` for line annotations. Keep `end_line` at or after the
  start.
- Use no unknown fields. Set new reviews to `draft`, with empty `annotations`
  and `decisions` lists unless the user supplied content for them.

The Patchflow application's `bin/patchflow validate` command is authoritative
when it is available.
