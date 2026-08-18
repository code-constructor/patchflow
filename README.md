# Patchflow

Patchflow is a local-first, agent-guided code-review application. It helps a
reviewer understand a change by turning a Git diff into an explicit review plan
instead of presenting every changed file in Git's default order.

An agent summarizes the change, identifies important execution and domain
paths, and proposes an order that builds understanding. The reviewer remains in
control: they can inspect the underlying diff, amend the plan, and make the
final decisions.

Patchflow treats the result of a review as durable project documentation. The
plan, explanations, diagrams, and review questions are stored inside the
reviewed repository so they can be committed alongside the code and understood
later.

> [!NOTE]
> Patchflow is in early development. The first local end-to-end review workflow
> is available, but its artifact schema and user experience are still evolving.

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
4. Read an agent-composed chapter built from prose, code, diff, callout,
   question, and diagram blocks.
5. Review syntax-highlighted split or unified diffs in that planned order.
6. Preserve the resulting documentation as a repository-local review artifact.

The commit SHAs are part of the evidence. If the target changes, the existing
artifact is stale and must be revised or regenerated rather than silently
describing different code.

## Review artifacts

The repository-local convention is:

```text
.patchflow/reviews/<review-id>/
  review.yaml
  overview.md
  diagrams/
  assets/
```

`review.yaml` is the machine-readable source of truth. It contains the
schema version, immutable source refs, change summary, ordered review steps,
priority rationales, narrative blocks, and review status. Markdown and Mermaid
source remain part of the artifact instead of existing only as rendered UI
state. Artifact v2 deliberately leaves interactive comments out while the
chapter language is established; v1 artifacts remain readable.

Generated `.patchflow` artifacts are excluded from the diff under review by
default, preventing Patchflow from reviewing its own output.

## First usable version

The initial vertical slice focuses on:

- selecting a local Git diff and resolving its source commits;
- creating and validating a machine-readable review artifact;
- rendering an overview with ordered, block-based review chapters;
- displaying syntax-highlighted split and unified diffs in the planned order;
- rendering Markdown and Mermaid diagrams.

Large-diff virtualization, GitHub pull-request import, specialized notebook or
image viewers, and extensive keyboard navigation are deliberately deferred
until the core workflow is useful. Oversized files currently receive a visible
placeholder so they do not block the rest of a review step.

## Technical foundation

- one Go 1.24+ application for the CLI, artifact engine, and local web UI;
- the standard library HTTP server and typed `html/template` view models;
- server-rendered HTML enhanced with Turbo and Stimulus;
- browser-native ES modules and an Import Map, with no Node or Vite pipeline;
- Diff2Html and Chroma for local diff rendering and syntax highlighting; and
- JSON Schema, templates, styles, and browser dependencies embedded into the
  self-contained binary with `go:embed`.

Patchflow is designed to run on the developer's machine. The initial product
does not require an account, hosted service, database, Node, Docker, or a cloud
dependency.

## Local development

Prerequisites:

- Git
- Go 1.24 or newer

Start the application directly from source:

```sh
bin/dev
```

The development server is available at <http://localhost:3000> by default.

### Docker and the local development proxy

Docker is an optional development path. When the shared `dev-proxy` Traefik
network is available, start Patchflow with:

```sh
dev-proxy up
docker compose up --build
```

Patchflow is then available at <http://patchflow.localhost>. No host port is
claimed by the application container; Traefik discovers it from Compose labels.

The host's `$HOME/Projects` directory is mounted at `/workspace`, allowing
Patchflow to review local projects. For example, host project
`$HOME/Projects/example` is selected in the container UI as
`/workspace/example`.

The **Browse…** button opens a server-side repository picker rooted at
`/workspace`, so container paths do not need to be entered by hand. In local
development the picker starts beside the preselected repository or in the
user's `Projects` directory.

The image runs as UID and GID `1000` by default so review artifacts remain owned
by the developer. Override these values on systems with different IDs:

```sh
PATCHFLOW_UID="$(id -u)" PATCHFLOW_GID="$(id -g)" docker compose up --build
```

Use `DEV_DOMAIN` to select another `*.localhost` hostname. This Docker workflow
is a convenience; Patchflow itself requires only its compiled binary and Git.

## Try the review workflow

Open Patchflow, select a local Git repository, and create a review from a base
ref such as `main` to a committed target such as `HEAD`. Patchflow resolves the
merge base and target SHA, creates a baseline review plan, and writes the result
to the selected repository.

The same workflow is available from the command line:

```sh
bin/patchflow create \
  --repository /absolute/path/to/repository \
  --base main \
  --target HEAD
```

Validate an existing artifact with:

```sh
bin/patchflow validate \
  /absolute/path/to/repository/.patchflow/reviews/<review-id>/review.yaml
```

Start the complete local application, optionally with a repository already
selected:

```sh
bin/patchflow serve --repository /absolute/path/to/repository
```

The server listens on <http://127.0.0.1:3000>. Build the self-contained binary
with `go build -o patchflow ./cmd/patchflow`; it contains the templates, CSS,
JavaScript modules, schema, and vendored browser libraries.

The repository also contains the Coding Agent skill
`create-patchflow-review` under `.agents/skills/`. It can analyze the committed
diff and enrich the baseline summary, review order, rationales, and Mermaid
documentation while preserving the recorded source SHAs.

## Verification

Run the complete local CI pipeline, including formatting, static analysis,
tests, and a production build:

```sh
bin/ci
```

## Code documentation

Every named Go function and method—including internal helpers and tests—has a
Go Doc comment that explains its purpose, contract, or important side effects.
Generate the complete package reference directly from the source with:

```sh
bin/docs
```

Pass an import path to inspect one package, for example
`bin/docs github.com/traqx-ai/patchflow/internal/gitrepo`. The CI pipeline checks
both the documentation convention and the documentation command, so new
undocumented functions cannot enter unnoticed.

## Design constraints

- **Local first:** network-backed features require an explicit product decision.
- **Git native:** review artifacts belong to the reviewed project and are meant
  to be versioned.
- **Agent guided, human controlled:** agents propose; reviewers inspect, amend,
  and decide.
- **Evidence preserving:** artifacts are tied to immutable commit SHAs.
- **Safe by default:** filesystem paths are untrusted and must never escape the
  selected repository.
- **Monorepo:** the Go application, embedded frontend, schemas, fixtures, docs,
  and agent skills evolve in the same repository.
