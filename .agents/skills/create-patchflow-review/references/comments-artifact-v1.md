# Patchflow comments artifact v1 reference

Discussion is stored in `.patchflow/reviews/<review-id>/comments.yaml`, separate
from the `review.yaml` narrative. The file has `schema_version: 1`, the matching
`review_id`, an `updated_at` UTC timestamp, and ordered `threads`.

Every thread and comment has a review-wide unique ID. A block target contains
`type: block` and `block_id`. A code target contains `type: code`, `block_id`,
repository `path`, `side: base | target`, the exact recorded `commit_sha`, and
an inclusive positive line range. Replies carry `reply_to` pointing to an
earlier comment in the same thread. Authors use `author_kind: human | agent`.

Use the Patchflow CLI instead of writing the file directly:

```sh
bin/patchflow comments --repository /path/to/repository /reviews/<review-id>
bin/patchflow show --repository /path/to/repository /reviews/<review-id>/comments/<comment-id>
bin/patchflow reply --repository /path/to/repository --author "Patchflow Agent" \
  --author-kind agent --body "<answer>" /reviews/<review-id>/threads/<thread-id>
bin/patchflow resolve --repository /path/to/repository /reviews/<review-id>/threads/<thread-id>
bin/patchflow validate /path/to/repository/.patchflow/reviews/<review-id>/comments.yaml
```

Pass `--reopen` to `resolve` to reopen a thread. Never replace human messages,
reuse IDs, move discussion into `review.yaml`, or alter its recorded source
anchor. A response should answer the referenced evidence and distinguish
confirmed behavior from inference or remaining uncertainty.
