# First vertical slice

## Outcome

The first Patchflow milestone reviews a committed local feature branch against
a committed base branch from beginning to end:

1. Select a local Git repository plus base and target refs.
2. Resolve the refs to immutable commit SHAs and determine the merge base.
3. Create and validate a repository-local review artifact.
4. Render an overview with an ordered set of review steps.
5. Compose each step from ordered prose, code, diff, callout, question, and
   diagram blocks.
6. Render syntax-highlighted split and unified diffs inside that narrative.
7. Validate the artifact contract and fixtures in the Go runtime.
8. Provide a Coding Agent skill that can create and enrich the same artifact.

This slice intentionally supports committed text changes in local repositories.
Working-tree changes, hosted pull-request import, interactive comment threads,
binary viewers, large-diff virtualization, and multi-user review are later work.

## Architecture boundaries

- The review artifact is the source of truth. Patchflow has no application
  database; repository selection is a local browser cookie.
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
- The Go application, schema, fixtures, documentation, embedded frontend, and
  Coding Agent skill live in one monorepo.

## Delivery steps

### 1. Review artifact v1 — complete

Define the versioned YAML contract, an example artifact, parsing, validation,
and safe atomic persistence. Record exact source SHAs, change metadata, ordered
steps, annotations, decisions, and status.

### 2. Local Git boundary — complete

Validate a selected repository, resolve refs, compute the merge base, list
changed files, and render unified diffs without invoking a shell. Reject invalid
refs, path traversal, and `.patchflow` paths.

### 3. Artifact creation — complete

Create a review directory from a commit comparison. Generate a useful baseline
plan with visible rationale and a Markdown overview.

### 4. Review overview — complete

Render source SHAs, change summary, status, changed files, and ordered review
steps directly from the artifact.

### 5. Planned diff — complete

Render one review step at a time with its rationale and unified diffs. Provide
simple previous/next navigation.

### 6. Artifact v2 blocks — complete

Replace the file-only chapter body with ordered narrative building blocks.
Keep v1 annotation support for old artifacts, but leave interaction and replies
out of v2 until the block language is proven.

### 7. Go artifact runtime and web application — complete

Validate fixtures against the canonical JSON Schema and semantic rules. Run the
complete repository selection, review creation, validation, and book-style UI
from one self-contained Go binary.

### 8. Coding Agent skill — complete

Teach `.agents/skills/create-patchflow-review` to compose the v2 block language,
avoid changing reviewed source files, and validate its result.

### 9. Rails-to-Go consolidation — complete

Remove the transitional Rails, Ruby, SQLite, and dual-runtime code. Serve typed
standard-library templates enhanced by Turbo and Stimulus, native ES modules,
plain CSS, and embedded browser dependencies without Node or a build pipeline.

## Acceptance scenario

Given a local repository with a `main` branch and a committed feature branch,
a reviewer can create a review, follow a rationale-backed story containing
explanation and focused code, switch between split and unified diffs, restart
Patchflow, and recover the review from a commit-ready
`.patchflow/reviews/<review-id>` artifact tied to the same base and target SHAs.
