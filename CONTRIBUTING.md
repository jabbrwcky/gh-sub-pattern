# Contributing

Issues and pull requests are welcome.

- Open an issue first for larger changes so the approach can be agreed on.
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages.
- Keep changes small and include tests for new logic.

## Development

Requires Go (version in `go.mod`) and the [GitHub CLI](https://cli.github.com/).

```sh
go vet ./...
go test ./...
go build && gh extension install .   # try it locally as `gh sub-pattern`
```

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).
Contributions are licensed under the [MIT License](LICENSE).
