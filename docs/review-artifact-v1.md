# Review artifact schema v1

`review.yaml` is the machine-readable source of truth for a Patchflow review.
All timestamps use ISO 8601 UTC strings. All paths use repository-relative `/`
separators and must not point into `.patchflow`.

## Document shape

```yaml
schema_version: 1
id: <review-id>
repository:
  name: <display-name>
source:
  base_ref: <ref requested by the user>
  target_ref: <ref requested by the user>
  base_sha: <40-character commit SHA used for the diff>
  target_sha: <40-character commit SHA used for the diff>
created_at: <ISO 8601 UTC timestamp>
updated_at: <ISO 8601 UTC timestamp>
status: draft | in_review | completed | stale
change:
  title: <short title>
  summary: <plain-text summary>
  files:
    - path: <repository-relative path>
      status: added | modified | deleted | renamed | copied | type_changed | unmerged | unknown
      previous_path: <required for renamed or copied files>
overview_path: overview.md
steps:
  - id: <step-id>
    title: <short title>
    priority: critical | high | medium | low
    rationale: <why this step belongs here>
    files:
      - <path present in change.files>
annotations:
  - id: <annotation-id>
    scope: overview | file | line
    body: <Markdown text>
    file_path: <required for file and line scopes>
    side: base | target # required for line scope
    start_line: <positive integer> # required for line scope
    end_line: <positive integer, not before start_line> # optional
    created_at: <ISO 8601 UTC timestamp>
decisions:
  - id: <decision-id>
    summary: <decision text>
    status: open | accepted | rejected
    rationale: <optional Markdown text>
    created_at: <ISO 8601 UTC timestamp>
```

## Invariants

- `schema_version` is exactly `1`.
- `id` and nested IDs contain only letters, digits, `_`, and `-` and start with
  a letter or digit.
- `base_sha` and `target_sha` are full lowercase Git commit SHAs. `base_sha` is
  the actual merge base when reviewing a branch against another branch.
- `overview_path` is exactly `overview.md` in schema v1.
- A review contains at least one changed file and one review step.
- Each changed path appears once. Every step contains at least one file, and
  every referenced file exists in `change.files`. Every changed file appears in
  at least one review step.
- Every annotation file exists in `change.files`.
- Artifact paths are relative, normalized, traversal-free, and outside the
  `.patchflow` directory.
- Unknown top-level or nested fields are invalid. Schema changes require a new
  schema version.

## Staleness

The source SHAs are immutable evidence. A branch moving after artifact creation
does not alter the artifact. When Patchflow explicitly compares the current
target ref with `source.target_sha` and they differ, it marks the review stale
or creates a revised artifact; it never silently reuses the old review for the
new commit.
