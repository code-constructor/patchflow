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
   question, diagram, and takeaway blocks.
5. Link directly to a stable block ID and carry syntax-highlighted split or
   unified diff state in the URL.
6. Discuss a whole block or selected source lines with humans and Coding Agents.
7. Preserve the resulting documentation and discussion as repository-local artifacts.

The commit SHAs are part of the evidence. If the target changes, the existing
artifact is stale and must be revised or regenerated rather than silently
describing different code.

## Review artifacts

The repository-local convention is:

```text
.patchflow/reviews/<review-id>/
  review.yaml
  comments.yaml
  overview.md
  diagrams/
  assets/
```

`review.yaml` is the machine-readable source of truth. It contains the
schema version, immutable source refs, change summary, ordered review steps,
priority rationales, narrative blocks, and review status. Markdown and Mermaid
source remain part of the artifact instead of existing only as rendered UI
state. `comments.yaml` is a separately versioned source of truth for block and
immutable source-range threads, including human and agent replies. It is created
lazily when the first comment is saved. V1 review artifacts remain readable.

Every v2 block has a review-wide unique semantic ID. Its copy control places a
stable `/reviews/<review-id>/blocks/<block-id>` path on the clipboard without
navigating, while `?diff=split|unified` carries shareable presentation state.
See the [review composition guide](docs/review-composition.md) for the narrative
and URL conventions.

Resolve a copied path back to its persisted chapter and block from the command
line. YAML is the default; pass `--format json` for programmatic use:

```sh
bin/patchflow show \
  --repository /absolute/path/to/repository \
  /reviews/<review-id>/blocks/<block-id>
```

Threads and individual comments use the same addressability contract:
`/reviews/<review-id>/threads/<thread-id>` and
`/reviews/<review-id>/comments/<comment-id>`. Copy controls do not navigate;
the paths can be passed back to `patchflow show` or an agent.

Comment authoring stays out of the reading flow until it is needed. Use the
speech-bubble action on a block for a block-wide thread. In code and diff
blocks, press a line's `+`, drag to the last relevant line, and release to open
the contextual composer for that immutable source range. Press and release on
one line to target only that line. Each composer stays anchored at its opening
position while the document scrolls, and multiple independent drafts can remain
open at once. Persisted source discussions appear as numbered line markers and
can be reopened contextually. A multi-line thread is rendered as one continuous
range with one marker, and the same thread cannot be opened in duplicate.
Human-authored comments can be edited without changing their stable IDs.
Patchflow pre-fills the reviewer name from the selected repository's effective
`git config user.name`; `Reviewer` is used only when Git has no configured name.
Comment creation, replies, edits, and resolution changes update their matching
discussion through targeted Turbo Streams. Open popovers, the surrounding
chapter, and the reader's scroll position remain in place, while the submitting
control exposes a compact loading state.

Generated `.patchflow` artifacts are excluded from the diff under review by
default, preventing Patchflow from reviewing its own output.

## First usable version

The initial vertical slice focuses on:

- selecting a local Git diff and resolving its source commits;
- creating and validating a machine-readable review artifact;
- rendering an overview with ordered, block-based review chapters;
- framing chapters with review questions, attention labels, decision gates, and
  addressable chapter takeaways;
- displaying syntax-highlighted split and unified diffs in the planned order;
- collapsing generated or mechanical evidence without removing it from scope;
- rendering Markdown and Mermaid diagrams.
- persisting addressable block and code-range discussions with replies.

Large-diff virtualization, GitHub pull-request import, specialized notebook or
image viewers, and extensive keyboard navigation are deliberately deferred
until the core workflow is useful. Oversized files currently receive a visible
placeholder so they do not block the rest of a review step.

## Technical foundation

- one Go 1.24+ application for the CLI, artifact engine, and local web UI;
- the standard library HTTP server and typed `html/template` view models;
- server-rendered HTML enhanced with Turbo and Stimulus;
- browser-native ES modules and an Import Map, with no Node or Vite pipeline;
- a token-based CSS theme whose palette, typography, surfaces, radii, shadows,
  and review layout can be changed through `:root` overrides;
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
Patchflow to review local projects. Herdr worktrees from `$HOME/.herdr/worktrees`
are mounted at `/workspace/worktrees` as well. Override that source when needed:

```sh
PATCHFLOW_WORKTREES_PATH=/another/worktree/root docker compose up --build
```

Any number of additional repository roots can be mounted beneath `/workspace`.
Copy `compose.mounts.example.yaml` to the Git-ignored `compose.override.yaml`,
then add one named target per host directory:

```yaml
services:
  app:
    volumes:
      - /host/client-projects:/workspace/client-projects
      - /host/other-worktrees:/workspace/other-worktrees
      # Required when linked-worktree metadata points at these host paths:
      - /host/client-projects:/host/client-projects
      - /host/other-worktrees:/host/other-worktrees
```

For example, `$HOME/Projects/example` is selected as `/workspace/example`, while
a Herdr worktree appears below `/workspace/worktrees`.
Linked Git worktrees store absolute paths in their `.git` metadata. The default
Compose setup therefore mirrors both standard roots at their original host
paths inside the container as well. Additional worktree mounts should follow
the same two-mount pattern shown above.

The Compose setup also mounts the host's XDG Git configuration read-only so
comment forms can use `git config user.name`. If your global Git configuration
lives elsewhere, point Patchflow at it explicitly:

```sh
PATCHFLOW_GIT_CONFIG_PATH="$HOME/.gitconfig" docker compose up --build
```

The **Browse…** button opens a server-side repository picker rooted at
`/workspace`, so container paths do not need to be entered by hand. In local
development the picker starts beside the preselected repository or in the
user's `Projects` directory.

Each opened repository receives a short URL namespace such as
`/repositories/4a1f…/reviews/<review-id>`. The absolute local path remains in an
HttpOnly cookie scoped to that namespace. Tabs can therefore keep reviews from
different projects open concurrently without changing one another's repository
selection.

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

The standalone discussion contract is validated the same way:

```sh
bin/patchflow validate \
  /absolute/path/to/repository/.patchflow/reviews/<review-id>/comments.yaml
```

List and inspect discussions, then answer one as a human or Coding Agent:

```sh
bin/patchflow comments --repository /absolute/path/to/repository \
  /reviews/<review-id>

bin/patchflow show --repository /absolute/path/to/repository \
  /reviews/<review-id>/comments/<comment-id>

bin/patchflow reply --repository /absolute/path/to/repository \
  --author "Patchflow Agent" --author-kind agent \
  --body "The service owns this boundary because …" \
  /reviews/<review-id>/threads/<thread-id>
```

See the [comments artifact contract](docs/comments-artifact-v1.md) for block
and code-range creation, replies, stable references, and resolution commands.

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
documentation while preserving the recorded source SHAs. It can also inspect
and answer persisted review comments through the CLI without browser automation.

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
