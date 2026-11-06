# 1. Immutable subject format by default

Date: 2026-11-06

## Status

Accepted

## Context

GitHub Actions OIDC tokens identify the repository in the `sub` claim. The legacy format
`repo:OWNER/REPO:…` uses names only. After a rename, transfer or deletion, someone else can
register the old name and obtain tokens that match existing trust policies.

GitHub introduced [immutable subjects](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens)
of the form `repo:OWNER@OWNER_ID/REPO@REPO_ID:…`. Repositories created after 2026-07-15 use it
by default; older ones keep the legacy format until they opt in. GitHub Enterprise Server does
not support it.

## Decision

Emit the immutable format by default. The extension resolves owner and repository IDs via the
API. `--mutable` selects the legacy format.

## Consequences

- Generated trust policies survive renames and cannot be claimed by a re-registered name.
- Resolving IDs costs no extra request: the repository lookup is already needed for canonical
  name casing.
- Older repositories that have not opted in would get non-matching patterns. [ADR 2](0002-repository-setting-overrides-format-flag.md)
  mitigates this by reading the repository's actual setting.
- GitHub Enterprise Server users must pass `--mutable`.
