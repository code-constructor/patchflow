# Patchflow

Patchflow is a local-first, agent-guided code-review application. It helps a
reviewer understand a change by turning a Git diff into an explicit review plan
instead of presenting every changed file in Git's default order.

An agent summarizes the change, identifies important execution and domain
paths, and proposes an order that builds understanding. The reviewer remains in
control: they can inspect the underlying diff, amend the plan, annotate the
change, and make the final decisions.

Patchflow treats the result of a review as durable project documentation. The
plan, explanations, diagrams, annotations, findings, and decisions are stored
inside the reviewed repository so they can be committed alongside the code and
understood later.

> [!NOTE]
> Patchflow is in early development. This repository currently contains the
> Rails foundation; the first end-to-end review workflow is the next milestone.

## Why Patchflow?

Large changes are rarely easiest to understand alphabetically or in raw Git
order. A useful review often starts with a domain boundary or entry point,
follows state and data through the system, and only then examines supporting or
presentation code.

Patchflow makes that reasoning visible. Every prioritization should have a
rationale, and important areas such as domain logic, interfaces, state changes,
dependencies, and security-sensitive paths should be explained before they are
critiqued.

## Intended workflow

1. Select a diff in a local Git repository.
2. Resolve and record the exact base and target commit SHAs.
3. Let an agent explain the change and propose an ordered review plan.
4. Review a unified text diff in that planned order.
5. Add overview, file, and line or range annotations.
6. Preserve the resulting documentation as a repository-local review artifact.

The commit SHAs are part of the evidence. If the target changes, the existing
artifact is stale and must be revised or regenerated rather than silently
describing different code.

## Review artifacts

The planned repository-local convention is:

```text
.patchflow/reviews/<review-id>/
  review.yaml
  overview.md
  diagrams/
  assets/
```

`review.yaml` is the machine-readable source of truth. It will contain the
schema version, immutable source refs, change summary, ordered review steps,
priority rationales, annotations, decisions, and review status. Markdown and
Mermaid source remain part of the artifact instead of existing only as rendered
UI state.

Generated `.patchflow` artifacts will be excluded from the diff under review by
default, preventing Patchflow from reviewing its own output.

## First usable version

The initial vertical slice focuses on:

- selecting a local Git diff and resolving its source commits;
- creating and validating a machine-readable review artifact;
- rendering an overview with ordered review steps;
- displaying a simple unified text diff in the planned order;
- supporting overview, file, and line or range annotations; and
- rendering Markdown and Mermaid diagrams.

Split diffs, large-diff virtualization, GitHub pull-request import, specialized
notebook or image viewers, and extensive keyboard navigation are deliberately
deferred until the core workflow is useful.

## Technical foundation

- Ruby 3.4 and Rails 8.1
- SQLite for application data, cache, jobs, and Action Cable
- Server-rendered HTML with Hotwire, Turbo, and Stimulus
- Importmap for JavaScript dependencies

Patchflow is designed to run on the developer's machine. The initial product
does not require an account, hosted service, Redis, PostgreSQL, Docker, or a
cloud dependency.

## Local development

Prerequisites:

- Ruby 3.4.10
- Bundler
- Git
- SQLite 3

Install dependencies, prepare the local databases, and start the application:

```sh
bin/setup
```

Or perform setup without starting the server:

```sh
bin/setup --skip-server
bin/dev
```

The development server is available at <http://localhost:3000> by default.

## Verification

Run the Rails test suite:

```sh
bin/rails test
```

Run the complete local CI pipeline, including formatting and security checks:

```sh
bin/ci
```

## Design constraints

- **Local first:** network-backed features require an explicit product decision.
- **Git native:** review artifacts belong to the reviewed project and are meant
  to be versioned.
- **Agent guided, human controlled:** agents propose; reviewers inspect, amend,
  and decide.
- **Evidence preserving:** artifacts are tied to immutable commit SHAs.
- **Safe by default:** filesystem paths are untrusted and must never escape the
  selected repository.
- **Rails native:** prefer Rails and browser capabilities over additional
  infrastructure or a client-side framework.
