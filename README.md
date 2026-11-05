# smol

A one-line Linux system-info CLI for login banners and shell prompts.

```text
$ smol
Ubuntu 24.04.1 LTS · 6.8.0-48-generic · 3d 4h 4m · 4.6 GiB / 15.6 GiB
```

## Build and use

Requires Go **1.26.10**, GNU Make, and Linux for live system information.

```sh
make build
./dist/smol
./dist/smol --format '{user}@{host} · {cpu} · {load}'
./dist/smol --color always --shell bash
./dist/smol --list-modules
```

`make build` produces `dist/smol`; install it somewhere on your `PATH` if desired.
`make run` runs the command directly. The module path is `github.com/rigerc/smol`.

Available modules: `cpu`, `host`, `kernel`, `load`, `mem`, `os`, `shell`,
`uptime`, and `user`. System files are read directly; smol does not spawn commands.
Modules run concurrently with a shared **20 ms** timeout (`--timeout 50ms` to
adjust it). Failed or unavailable modules disappear with their preceding
separator. Prefix and suffix literals remain. Use `{{` and `}}` for literal braces.
Output has one trailing newline. Control characters in module values are removed;
format strings containing control characters are rejected.

## Configuration

```sh
mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/smol"
./dist/smol --print-default-config > "${XDG_CONFIG_HOME:-$HOME/.config}/smol/config.toml"
./dist/smol --check-config
```

The default file is `$XDG_CONFIG_HOME/smol/config.toml`, falling back to
`$HOME/.config/smol/config.toml`. Use `--config /path/to/config.toml` to select
another file. No configuration file is required.

```toml
format = "{os} · {kernel} · {uptime} · {mem}"
color = "auto"

[style]
os = { fg = "cyan", bold = true }
mem = { fg = "yellow" }
```

Styles override defaults per module. Foreground colors are `black`, `red`,
`green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, their `bright-` variants,
or `""` for the terminal default. Unknown keys, modules, and colors are rejected.
TOML decoding uses [go-toml/v2](https://github.com/pelletier/go-toml/tree/v2.2.3),
the sole runtime dependency, to support the supplied TOML configuration syntax
and diagnostics without maintaining a custom parser.

Flags `--format` and `--color` override configuration values. Color modes are
`auto`, `always`, and `never`; auto suppresses colors for redirected output,
`NO_COLOR`, or `TERM=dumb`. Explicit `always` overrides these choices.
`--shell none|bash|zsh|fish` controls prompt escape conventions; it does not
install shell integration. `sample.env` documents environment inputs. The app
does not load `.env` files or use the template's former `APP_ENV`/`LOG_LEVEL` settings.

A malformed or unreadable configuration silently falls back to defaults for
normal output. Run `--check-config` to see errors, including source line numbers
for TOML syntax and unknown-key errors. Exit codes: **0** for success/help,
**1** for configuration-check or output-write failure, **2** for invalid CLI input.
`--version` prints the build version (default `dev`).

## Development

```sh
make fix
make check
```

The Makefile selects Go 1.26.10. `make check` runs formatting checks, vet,
the pinned custom golangci-lint **v2.10.1** with anti-slop-go **v1.3.0**,
race-enabled tests, an **80%** internal statement coverage gate, and a build.
The first lint run requires the `golangci-lint` builder and network access.
Use `make lint`, not stock `golangci-lint run`, to load the plugin.
Run `make audit` with `govulncheck` before dependency upgrades or releases.

- `cmd/app/`: process entry point, built as `smol`.
- `internal/app/`: CLI execution with injected filesystem, environment, and writers.
- `internal/config/`: TOML defaults, strict parsing, and validation.
- `internal/modules/`: nine built-in system-info modules.
- `internal/render/`: format parsing, concurrent fetches, separators, and color escapes.
- `testdata/`: x86 and ARM Linux fixtures used by deterministic tests.

Caching, `smol init`, package/GPU/battery modules, and width truncation are not
implemented in the supplied app or this adaptation.

The template's [BSD-2-Clause license](LICENSE) is retained.
