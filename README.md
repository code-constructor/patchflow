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
   question, diagram, image, and takeaway blocks.
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
state. Raster screenshots and other visual evidence live below `assets/` and
render as responsive, fullscreen-capable image blocks. `comments.yaml` is a separately versioned source of truth for block and
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

Repository review lists separate current evidence from collapsed stale
history. `Draft` remains the lifecycle status of an unfinished review; `Stale`
means its target ref no longer resolves to the recorded target commit. The
newest non-stale review is highlighted first. Stale entries remain available
as immutable history and show their unresolved-thread count.

Comments are never copied automatically into a replacement review. Each
`comments.yaml` remains beside its original `review.yaml`, and code threads are
anchored to that review's immutable commit SHA and block IDs. Automatic copying
would make old line anchors appear valid against different source. Unresolved
threads therefore remain visible on stale history; carrying one forward must be
an explicit re-anchoring into a new thread with a new ID while preserving the
old conversation as evidence.

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
- streaming every changed file down one continuous, tree-addressable page and
  lazy-loading its diff near the viewport, with a collapsible file tree for
  full-width code review;
- persisting personal Viewed progress in the local user configuration, with
  collapsed viewed files in Files changed and non-collapsing status in the plan;
- collapsing generated or mechanical evidence without removing it from scope;
- rendering Markdown, Mermaid diagrams, and safe raster image blocks;
- persisting addressable block and code-range discussions with replies.

Large-diff virtualization beyond viewport loading, GitHub pull-request import,
specialized notebook viewers, and extensive keyboard navigation are deliberately deferred
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
The container also exports the server's config path to CLI subprocesses. A
Coding Agent can therefore create and register a review without opening the
repository picker or assembling SHAs, for example:

```sh
docker compose exec -w /workspace/example app \
  patchflow create --format json
```

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
`/repositories/4a1f…/reviews/<review-id>`. The absolute local path remains in a
versioned user configuration rather than the URL or browser session. Native
execution uses `$XDG_CONFIG_HOME/patchflow/config.json` (normally
`~/.config/patchflow/config.json`). The Docker development setup uses
`/workspace/.patchflow/config.json`, persisted on the host as
`$HOME/Projects/.patchflow/config.json`; override the container path with
`PATCHFLOW_DOCKER_CONFIG_PATH` when required.

Tabs and even separate browsers can therefore keep reviews from different
projects open concurrently. The root page lists all reachable repositories
remembered by that configuration beneath the picker, and every repository page
links back to the workspace overview. Review pages also show a GitHub shortcut
when a local remote points to `github.com`. An explicit `refs/pull/<number>/head`
or `refs/pull/<number>/merge` target links to that pull request; other target
refs link to the repository. This is derived locally and does not require a
GitHub login or network request. Existing repository cookies from earlier
Patchflow versions are imported into the file on first use.

The settings document starts with repository history and is intentionally
extensible for future preferences:

```json
{
  "version": 1,
  "repositories": [
    {
      "path": "/workspace/example",
      "last_opened_at": "2026-08-19T12:00:00Z"
    }
  ],
  "review_progress": [
    {
      "repository_path": "/workspace/example",
      "review_id": "20260819-120000-deadbeef",
      "target_sha": "0123456789abcdef0123456789abcdef01234567",
      "viewed_files": ["cmd/patchflow/main.go"]
    }
  ]
}
```

An explicit path is also available outside Docker:

```sh
patchflow serve --config /path/to/config.json
```

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
to the selected repository. Within a review, the global **Review plan** and
**Files changed** tabs switch between the guided narrative and a classic file
tree backed by the same immutable commits. Files form one continuous page;
their Turbo Frames load near the viewport, and selecting a tree path scrolls to
its stable URL without throwing away the surrounding page. Turbo navigation
remembers the last resource and scroll position independently for both views in
the current browser tab. A Viewed toggle collapses that file in the classic
view while the guided plan keeps its code visible and shows only the shared
completion state. The state survives browsers and restarts in the local config.

The same workflow is available from the command line. Run it inside the target
repository and Patchflow detects the local default branch, uses `HEAD` as the
target, resolves both immutable commits, creates a baseline artifact, validates
its Git evidence, and remembers the repository for the local UI:

```sh
bin/patchflow create --format json
```

Use `--repository`, `--base`, or `--target` when the command runs outside the
reviewed worktree or automatic base discovery is not the intended comparison.
`--config` selects the same settings file as a separately running Patchflow
server. Successful JSON output includes the artifact path, browser route,
repository path, refs, and full base/target SHAs. Failures return a non-zero
status and an `errors` array intended for Coding Agents.

After an agent curates `review.yaml`, validate the complete stored review with:

```sh
bin/patchflow validate \
  --format json \
  /absolute/path/to/repository/.patchflow/reviews/<review-id>/review.yaml
```

Validation is repository-backed, not only syntactic. It checks the schema and
semantic IDs, proves that both recorded SHAs are available commits, compares
the complete changed-file inventory with Git, verifies code-block paths and
line ranges, and confirms that `overview.md`, Mermaid sources, and raster image
assets are readable. The repository is inferred from the canonical
`.patchflow/reviews` path; `--repository PATH` is available for explicit
diagnostics. A review outside that canonical directory is rejected because the
UI could not discover it.

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
