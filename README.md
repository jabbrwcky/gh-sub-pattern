# gh-sub-pattern

A [GitHub CLI](https://cli.github.com/) extension that prints `sub` claim patterns for
[GitHub Actions OIDC tokens](https://docs.github.com/en/actions/reference/security/oidc),
ready to paste into a cloud provider's trust policy (AWS `StringLike`, GCP, Vault, …).

## Install

```sh
gh extension install jabbrwcky/gh-sub-pattern
# or from a local checkout
go build && gh extension install .
```

## Usage

```
gh sub-pattern [OWNER/REPO] [flags]
```

Without `OWNER/REPO` the repository of the current directory is used. Every flag value
produces one pattern; patterns are printed one per line on stdout, warnings go to stderr.

| Flag | Meaning | Example → pattern suffix |
|---|---|---|
| `-e, --environment` | Deployment environment | `-e prod` → `environment:prod` |
| `-r, --ref` | Exact branch or full ref | `-r main` → `ref:refs/heads/main` |
| `--refs` | Branch prefix, `*` appended | `--refs release/` → `ref:refs/heads/release/*` |
| `-t, --tag` | Exact tag | `-t v1.2.0` → `ref:refs/tags/v1.2.0` |
| `--tags` | Tag prefix, `*` appended | `--tags v1.` → `ref:refs/tags/v1.*` |
| `--mutable` | Legacy `OWNER/REPO` segment (see below) | |

All target flags are repeatable and can be combined. Refs not starting with `refs/heads/`
or `refs/tags/` get `refs/heads/` prepended. `-r pull_request` yields the `pull_request`
subject. Refs are not checked for existence, so patterns for future tags work.

## Examples

```sh
# Branch of the current repository
$ gh sub-pattern -r main
repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main

# Several patterns at once
$ gh sub-pattern octo-org/octo-repo -e production --refs release/ --tags v
repo:octo-org@123456/octo-repo@456789:environment:production
repo:octo-org@123456/octo-repo@456789:ref:refs/heads/release/*
repo:octo-org@123456/octo-repo@456789:ref:refs/tags/v*

# Pull requests and an exact tag
$ gh sub-pattern octo-org/octo-repo -r pull_request -t v2.0.0
repo:octo-org@123456/octo-repo@456789:pull_request
repo:octo-org@123456/octo-repo@456789:ref:refs/tags/v2.0.0

# Legacy subject format
$ gh sub-pattern octo-org/octo-repo --mutable -r main
repo:octo-org/octo-repo:ref:refs/heads/main

# Collect patterns into a JSON array for a policy
$ gh sub-pattern -r main --tags v | jq -R . | jq -s .
```

### Interactive mode

Without any target flag and with a terminal attached, the extension lists the repository's
environments, branches, tags, `pull_request` and derived prefix globs (e.g. `release/*`) in
[fzf](https://github.com/junegunn/fzf). Pick one, or type your own ref/glob and press enter.
Without fzf it falls back to gh's built-in selector (no free-text input).

## Subject format

GitHub is moving to [immutable subjects](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens)
that pin the owner and repository by ID: `repo:OWNER@OWNER_ID/REPO@REPO_ID:…`.
The extension resolves both IDs and the canonical name casing via the API.

Which format is used:

1. The repository's OIDC setting (`use_immutable_subject`), if the token can read it.
2. Otherwise immutable, or legacy `repo:OWNER/REPO:…` with `--mutable`.

If the repository uses a custom `sub` template, a warning is printed: the patterns follow the
default template and will not match. GitHub Enterprise Server has no immutable subjects; use
`--mutable` there.

Rationale: [ADR 1](docs/adr/0001-immutable-subject-format-by-default.md),
[ADR 2](docs/adr/0002-repository-setting-overrides-format-flag.md).

## Releases

Pushing a `v*` tag builds binaries for all platforms via
[gh-extension-precompile](https://github.com/cli/gh-extension-precompile), with GitHub build
provenance attestations and cosign keyless signatures (`<platform>.sigstore.json`). Verify a
download, e.g. `gh release download v0.1.3 -R jabbrwcky/gh-sub-pattern -p "darwin-arm64*"`:

```sh
gh attestation verify darwin-arm64 --repo jabbrwcky/gh-sub-pattern

cosign verify-blob darwin-arm64 \
  --bundle darwin-arm64.sigstore.json \
  --certificate-identity-regexp "^https://github.com/jabbrwcky/gh-sub-pattern/" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) and
[SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
