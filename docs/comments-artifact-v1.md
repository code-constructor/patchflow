# Patchflow comments artifact v1

`comments.yaml` is the Git-native discussion attached to one Patchflow review.
It is stored beside `review.yaml` and created only when the first thread is
opened. Keeping it separate prevents interactive conversation from rewriting
the agent-authored review narrative.

The canonical machine-readable contract is
[`schema/patchflow-comments-v1.schema.json`](../schema/patchflow-comments-v1.schema.json).

## Anchors and identity

Every thread and every comment, including replies, has a unique stable ID. A
thread targets either:

- a whole narrative block by its stable `block_id`; or
- source evidence by `block_id`, repository `path`, `side`, exact
  `commit_sha`, and an inclusive `start_line`/`end_line` range.

Patchflow derives `commit_sha` from the immutable base or target recorded in
`review.yaml`; browser and CLI input cannot choose a different commit. Code
excerpt ranges must stay inside their block. Diff comments must reference the
same changed path as their diff block.

Replies are stored in thread order and use `reply_to` to identify an earlier
comment in the same thread. `resolved` records workflow state without deleting
the conversation.

```yaml
schema_version: 1
review_id: 20260819-090000-deadbeef
updated_at: "2026-08-19T09:15:00Z"
threads:
  - id: thread-88f2e1983207e334
    target:
      type: code
      block_id: request-handler
      path: internal/web/server.go
      side: target
      commit_sha: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      start_line: 347
      end_line: 354
    resolved: false
    comments:
      - id: comment-2a570feaafeed0ac
        author: Alex
        author_kind: human
        body: Why does this handler own the lookup?
        created_at: "2026-08-19T09:14:00Z"
      - id: comment-cf234453e5a6bd6d
        author: Patchflow Agent
        author_kind: agent
        body: It should not; the store already owns that boundary.
        created_at: "2026-08-19T09:15:00Z"
        reply_to: comment-2a570feaafeed0ac
```

## CLI lifecycle

Validate and list the discussion:

```sh
bin/patchflow validate /path/to/.patchflow/reviews/<review-id>/comments.yaml
bin/patchflow comments --repository /path/to/repository /reviews/<review-id>
```

Open a whole-block thread:

```sh
bin/patchflow comment --repository /path/to/repository \
  --author Alex --body "Why is this boundary here?" \
  /reviews/<review-id>/blocks/<block-id>
```

Add `--path`, `--side`, `--start-line`, and `--end-line` to address source
lines. Reply as an agent and resolve the thread:

```sh
bin/patchflow reply --repository /path/to/repository \
  --author "Patchflow Agent" --author-kind agent \
  --body "This keeps the write policy in one place." \
  /reviews/<review-id>/threads/<thread-id>

bin/patchflow resolve --repository /path/to/repository \
  /reviews/<review-id>/threads/<thread-id>
```

Pass `--reopen` to `resolve` to reopen a thread. `patchflow show` accepts copied
block, thread, and comment paths and emits YAML by default or JSON with
`--format json`.
