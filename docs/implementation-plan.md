# First vertical slice

## Outcome

The first Patchflow milestone reviews a committed local feature branch against
a committed base branch from beginning to end:

1. Select a local Git repository plus base and target refs.
2. Resolve the refs to immutable commit SHAs and determine the merge base.
3. Create and validate a repository-local review artifact.
4. Render an overview with an ordered set of review steps.
5. Render a unified text diff for each step.
6. Add overview, file, and line or range annotations.
7. Preserve all review state in the artifact across application restarts.
8. Provide a Coding Agent skill that can create and enrich the same artifact.

This slice intentionally supports committed text changes in local repositories.
Working-tree changes, hosted pull-request import, binary viewers, split diffs,
large-diff virtualization, and multi-user review are later work.

## Architecture boundaries

- The review artifact is the source of truth. SQLite may hold local convenience
  state, but not the durable review itself.
- Git commands live behind a service boundary and receive argument arrays, not
  shell-interpolated commands.
- Refs resolve to SHAs before diffing. Artifacts never silently follow a moving
  branch.
- Paths from requests and artifacts are untrusted. Review files stay below the
  selected repository's `.patchflow/reviews` directory, and diff paths stay
  within the selected repository.
- Plan generation is an interface. The first application-generated plan is a
  deterministic baseline; a Coding Agent can replace it with a richer plan by
  producing the same artifact format.
- Generated `.patchflow` content is excluded from the reviewed diff by default.

## Delivery steps

### 1. Review artifact v1

Define the versioned YAML contract, an example artifact, parsing, validation,
and safe atomic persistence. Record exact source SHAs, change metadata, ordered
steps, annotations, decisions, and status.

### 2. Local Git boundary

Validate a selected repository, resolve refs, compute the merge base, list
changed files, and render unified diffs without invoking a shell. Reject invalid
refs, path traversal, and `.patchflow` paths.

### 3. Artifact creation

Create a review directory from a commit comparison. Generate a useful baseline
plan with visible rationale and a Markdown overview.

### 4. Review overview

Render source SHAs, change summary, status, changed files, and ordered review
steps directly from the artifact.

### 5. Planned diff

Render one review step at a time with its rationale and unified diffs. Provide
simple previous/next navigation.

### 6. Annotations

Persist overview, file, and base/target line-range annotations back into
`review.yaml` and show them in the relevant context.

### 7. Walking Skeleton test

Create a temporary Git repository with base and feature commits, drive the HTTP
workflow, add an annotation, reload the artifact, and verify that `.patchflow`
output does not enter the reviewed file set.

### 8. Coding Agent skill

Add `.agents/skills/create-patchflow-review`. The skill must resolve source
commits, explain the change before critiquing it, prioritize important paths,
write the canonical artifact structure, avoid changing reviewed source files,
and validate its result.

## Acceptance scenario

Given a local repository with a `main` branch and a committed feature branch,
a reviewer can create a review, see a rationale-backed order, inspect each
unified diff, annotate a target line, restart Patchflow, and recover the review
from a commit-ready `.patchflow/reviews/<review-id>` artifact tied to the same
base and target SHAs.
