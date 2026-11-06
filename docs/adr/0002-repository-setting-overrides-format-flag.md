# 2. Repository setting overrides the format flag

Date: 2026-11-06

## Status

Accepted

## Context

Whether a repository issues immutable or legacy subjects ([ADR 1](0001-immutable-subject-format-by-default.md))
depends on its creation date and opt-in state, which users often do not know. A pattern in the
wrong format silently never matches.

`GET /repos/{owner}/{repo}/actions/oidc/customization/sub` returns `use_immutable_subject`.
The field was observed in API responses but not documented at the time of writing. The request
can fail for lack of permissions, and the field is absent on GitHub Enterprise Server.

## Decision

If the field can be read, it decides the format, and a note goes to stderr when it differs from
what the flags asked for. Only when it cannot be read do the default and `--mutable` apply.

## Consequences

- Output matches the repository's actual format without the user having to know it.
- Users cannot force a format against the repository setting, e.g. to prepare a policy before
  opting in. Revisit with an explicit override flag if that need appears.
- If GitHub removes or renames the field, behaviour degrades to the flag-based default rather
  than failing.
- Inheritance of an organization-level setting is not verified; the repository endpoint is
  assumed to report the effective value.
