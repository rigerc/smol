# smol

## Setup

Use Go 1.26.10 (as specified in `go.mod`) and the custom `golangci-lint` v2.10.1 with `anti-slop-go` v1.3.0.
Application packages use the Go standard library plus `go-toml/v2` for TOML
configuration parsing. Do not add dependencies without a specific reason.

## Verification

Before opening a pull request, run:

```sh
make check
```

`make check` enforces `gofmt`, `go vet`, anti-slop Go analysis and standard lint, race-enabled tests,
an 80% statement coverage floor on business packages, and a compiled binary.
Use `make fix` for formatting. Do not use plain `golangci-lint run`: it lacks
the anti-slop module plugin. `make lint` builds and runs `./custom-gcl` from
`.custom-gcl.yml` with the pinned plugin. Run `make audit` with `govulncheck` before
dependency upgrades or releases.

## Structure

- `cmd/app/` is the composition root; the executable is `dist/smol`.
- `internal/app/` runs the CLI with injected runtime inputs and writers.
- `internal/config/` loads and validates TOML configuration.
- `internal/modules/` fetches system info; `internal/render/` formats output.
- `sample.env` documents environment inputs for config discovery, modules, and colors.
- `.custom-gcl.yml` pins the custom linter and anti-slop module.
- `.golangci.yml` configures Go linters and the enabled anti-slop rules.

## Code style

Return errors rather than panicking. Keep I/O at boundaries and inject loggers,
writers, and environment lookups; avoid global mutable state. Use `slog` instead
of ad hoc print logging. Add tests for invalid input and failure paths. Keep
`sample.env` and README configuration documentation aligned with behavior.

## Boundaries

Keep changes scoped to the task. Ask first before adding deployment,
publication, or infrastructure automation. Never commit `.env`, credentials,
coverage output, or binaries. Do not bypass checks to make CI green.
