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

Available modules: `cpu`, `face`, `host`, `kernel`, `load`, `mem`, `os`, `shell`,
`uptime`, and `user`. System files are read directly; smol does not spawn commands.
Modules run concurrently with a shared **20 ms** timeout (`--timeout 50ms` to
adjust it). Failed or unavailable modules disappear with their preceding
separator. Prefix and suffix literals remain. Use `{{` and `}}` for literal braces.
Static output has one trailing newline. Control characters in module values are removed;
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
TOML decoding uses [go-toml/v2](https://github.com/pelletier/go-toml/tree/v2.2.3)
to support the supplied TOML configuration syntax
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

## Faces and animation

Add `{face}` to your format to show an asciimoji from
[faceroll](https://github.com/rigerc/faceroll/tree/main/go). These text faces
include Unicode glyphs; a suitable terminal font is needed. Strict ASCII-only
filtering is not implemented.

```sh
./dist/smol --format '{face} {host} · {mem}'
./dist/smol --format '{face} {host}' --face-preset kitty --face-state happy
./dist/smol --format '{face} {host}' --face-seed my-machine
./dist/smol --format '{face} {host}' --animate --duration 5s
./dist/smol --list-face-presets
./dist/smol --list-face-states
```

The default format stays unchanged. Faces default to the `idle` expression and
a repeatable generated character with seed `"smol"`. The same seed and pinned
faceroll version produce the same character. Repeated `{face}` placeholders use
the same character and synchronized frame.

```toml
format = "{face} {user}@{host} · {mem}"

[face]
preset = "kitty"
state = "idle"
# Alternatively, replace preset with seed = "my-machine".
# seed = "" is also a valid deterministic seed.

[style]
face = { fg = "cyan", bold = true }
```

CLI face options override configuration. `--face-preset` replaces a configured
seed; `--face-seed` replaces a configured preset. Supplying both selectors on the
CLI, or both in the file, is an error. `--face-state` selects the expression.
Face settings are validated even when the format does not include `{face}`.

Animation is enabled only through `--animate`. It redraws the complete line using
faceroll frame timings; system information is fetched once at startup.
`--duration` accepts a positive Go duration such as `5s` or `500ms`. Omit it to
animate until Ctrl+C or SIGTERM. Single-frame expressions print once and exit.

Playback requires a terminal with known width and enough columns for every
frame. Piped output, `TERM=dumb`, and insufficient or unknown width produce a
single static line. Animation stops if a terminal resize makes the line too wide.
Width is estimated using faceroll's terminal cell-width rules; fonts can vary.
Animation is rejected when `{face}` is absent or `--shell` is set to anything
other than `none`; shell-prompt embedding uses static faces.

Playback restores the cursor and ends the final frame with a newline on duration
expiry, interruption, or resize. It attempts the same cleanup after a write
failure. SIGINT exits **130**, SIGTERM exits **143**, and output failure exits
**1**. Faceroll generation and animation use the Go library directly, with no
external runtime or subprocess. Its dependency is pinned in `go.mod`.

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
- `internal/render/`: format parsing, module snapshots, separators, and color escapes.
- `internal/face/`: faceroll character selection, validation, and discovery.
- `internal/playback/`: synchronous animation with checked writes and terminal cleanup.
- `testdata/`: x86 and ARM Linux fixtures used by deterministic tests.

Caching, `smol init`, package/GPU/battery modules, and width truncation are not
implemented in the supplied app or this adaptation.

The template's [BSD-2-Clause license](LICENSE) is retained.
