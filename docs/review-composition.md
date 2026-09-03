# Composing a Patchflow review

Patchflow reviews are guided explanations of a committed change, not decorated
file lists. Their order should help a reviewer build the right mental model
before asking them to judge implementation details.

## Research basis

The initial conventions draw from established review guidance:

- Google's review navigation starts with the broad purpose, then the most
  important design area, followed by the remaining change in a logical order.
- GitHub recommends small, focused changes; a description that explains why,
  what, and the result; explicit focus areas; and guidance when review order
  matters.
- Google's change-description guidance treats the summary and rationale as
  durable history, including relevant background, shortcomings, and decisions
  that source code cannot reveal.
- Review guidance consistently treats tests, security, system context, and code
  health as evidence that humans must assess, not boxes that automation alone
  can check.

Primary sources:

- [Navigating a CL in review](https://google.github.io/eng-practices/review/reviewer/navigate.html)
- [Writing good CL descriptions](https://google.github.io/eng-practices/review/developer/cl-descriptions.html)
- [Small CLs](https://google.github.io/eng-practices/review/developer/small-cls.html)
- [What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html)
- [Helping others review your changes](https://docs.github.com/en/pull-requests/concepts/helping-others-review-your-changes)

## Overview contract

Write the overview for a capable reviewer who has no author context. Include:

1. **Purpose:** the user, product, or engineering problem being solved.
2. **Outcome:** the observable before-and-after behavior.
3. **Approach:** the main design decision and why it fits the problem.
4. **Scope:** important non-goals, deliberately deferred work, and generated or
   mechanical portions of the diff.
5. **Impact:** changed state, interfaces, dependencies, trust boundaries,
   compatibility, operations, or migration behavior.
6. **Evidence:** relevant tests, checks, benchmarks, screenshots, or manual
   verification.
7. **Uncertainty:** known tradeoffs, assumptions, limitations, and residual
   risks. Resolve product, architecture, and implementation decisions during
   development rather than deferring them into the review.
8. **Reading path:** why the proposed chapter order is the fastest route to
   understanding.

Do not restate filenames or commit messages. Preserve author context that the
code cannot express and make important omissions explicit.

## Chapter order

Choose conceptual chapters before assigning files. A useful default order is:

1. establish the main product, domain, or architecture boundary;
2. define the contract or behavior the change promises;
3. follow the primary execution or data path from entry point to result;
4. examine state changes, failure paths, and security-sensitive boundaries;
5. inspect integrations, presentation, configuration, and operational effects;
6. close with cross-cutting verification or documentation that was not already
   shown beside the behavior it proves.

Move tests earlier when they are the clearest specification of intended
behavior. Keep a migration or compatibility layer early when every later
chapter depends on understanding it. Never force a change into this sequence
when another order teaches it more clearly.

Each chapter needs a clear learning objective and a rationale for appearing at
that point. Prefer several small, coherent chapters to one exhaustive chapter
or one chapter per directory. Keep the changed-file inventory complete, but do
not force every path into visible code or diff evidence. If any path is omitted
from visible evidence, close with a compact coverage chapter that names it and
explains why it was omitted.

Treat commit history as a clue, not as the table of contents. Coherent commits
can reveal intent or separate a mechanical refactor from behavior, but Patchflow
reviews the final base-to-target change and must still make sense after squash or
rebase. If the diff contains several unrelated conceptual changes, call that out
as a scope risk instead of inventing a narrative that hides the review cost.

Build the journey through progressive disclosure: establish purpose, outcome,
system boundaries, and the main architectural approach first; then move through
contracts and primary flows before zooming into non-obvious implementation
details. Size the journey by conceptual complexity, novelty, and risk rather
than lines or files changed. Its goal is a navigable mental model, not exhaustive
line-by-line knowledge.

Represent that behavior directly in the artifact when it adds signal:

- `review_question` may focus verification, but must not defer an unresolved
  development decision into Patchflow;
- `attention` names the kind of judgment required;
- a final `takeaway` block states the addressable mental model that closes the
  chapter; and
- `collapsed: true` removes supporting diff noise from the first reading pass
  without hiding its existence.

The UI treats a critical chapter with a review question as a decision gate. It
also presents a sticky block navigator on wide screens; its controls scroll in
place and do not mutate URL or browser history.

## Block grammar

Build chapters as claim-and-evidence sequences:

1. Orient with `prose`: what the reviewer is about to learn and why it matters.
2. Use `code` only when unchanged or surrounding context establishes the
   contract needed to understand the change.
3. Use `diff` as evidence for the actual change; focus it when the whole file
   obscures the relevant decision.
4. Follow evidence with a `callout` when a consequence, risk, assumption, or
   tradeoff deserves explicit attention.
5. Do not use `question` to solicit a product, architecture, or implementation
   decision. Resolve that choice during development, then explain the selected
   option and rationale with prose or a callout.
6. Use `diagram` when relationships, ownership, or sequence are materially
   clearer visually than in prose.
7. Use `image` when a screenshot or rendered result is itself review evidence;
   its caption should direct attention rather than merely repeat its alt text.
8. Close with `takeaway` when the reviewer needs an explicit mental model before
   moving to the next chapter.

Not every sequence needs every block type. Avoid decorative diagrams, repeated
introductions, prose that translates syntax, and large excerpts without a stated
review purpose. Put tests close to the behavior they prove instead of collecting
them mechanically at the end.

Not every changed file needs visible evidence. Keep `change.files` and step
`files` metadata complete for Git-native traceability, but omit code and diff
blocks that add no new understanding. When omissions exist, the final
`scope-and-coverage` chapter must list each omitted path and a specific reason;
large mechanical groups may use an exact path pattern and count. Do not add an
empty coverage chapter when the visible journey already represents every path.

## Stable block references

Every block has an ID that is unique across its review. Choose a short semantic
ID such as `repository-path-boundary` or `rendered-diff-evidence`, not a
position such as `block-7`. Keep the ID when wording changes or the block moves
to another chapter. Never reuse a removed ID for unrelated content.

Patchflow exposes two forms of address:

```text
/reviews/<review-id>/blocks/<block-id>
/reviews/<review-id>/steps/<step-id>?diff=unified
```

The first is the stable reference path and resolves the block's current chapter.
The UI copies this path from a small button without navigating or reloading the
page. When opened directly, the same path renders the containing chapter and
marks the addressed block without redirecting through a fragment URL or
introducing asynchronous scroll behavior. The second addresses the
chapter with shareable presentation state. Giving an agent either the block ID
or reference path is enough to identify the corresponding YAML block
unambiguously.

Resolve a copied path without scraping the web UI:

```sh
bin/patchflow show --repository /path/to/repository \
  /reviews/<review-id>/blocks/<block-id>
```

The command prints review and chapter context plus the persisted block as YAML,
or as JSON with `--format json`.

## URL state and durable state

Use each URL component deliberately:

- **Path:** durable resource identity and focus such as review, chapter, or
  block.
- **Query:** shareable presentation choices such as `diff=split|unified`.

Do not persist URL-representable presentation state in a database or hidden
browser storage. Use `history.replaceState` for choices that refine the current
view without creating a new navigation entry.

Keep semantic review content in the repository-local artifact: explanations,
ordered blocks, diagrams, status, and immutable base and target SHAs. URLs
address this content; they do not replace it. Encoding prose, absolute local
paths, or review decisions in URLs would expose machine-specific information,
produce unstable links, and discard the Git-native record that Patchflow is
designed to create.
