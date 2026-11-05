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

System files are read directly; smol does not spawn commands or make network
requests. The default format is unchanged. Additional modules are opt-in:

| Module | Default text | Fields (`{module:field}`) |
| --- | --- | --- |
| `os` | OS name | `id`, `version` |
| `kernel` | Release | `arch`, `release`, `name` |
| `uptime` | Duration | `days`, `hours`, `minutes`, `seconds` |
| `mem` | Used / total | `used`, `free`, `available`, `total`, `percent`, `bar` |
| `cpu` | Model (threads) | `threads`, `cores`, `freq`, `temp` |
| `load` | Three load averages | `percent` (one-minute load / logical CPUs) |
| `host`, `user`, `shell` | Hostname, user, shell | — |
| `face` | Configured asciimoji | — |
| `swap`, `disk` | Used / total | `used`, `free`, `total`, `percent`, `bar` |
| `battery` | Charge percentage | `percent`, `status` |
| `terminal` | Terminal emulator | — |
| `desktop` | Desktop name | `wm`, `session` |
| `virt` | Virtualization environment | — |
| `time` | Local time | — |
| `processes`, `init` | Process count, init name | — |
| `machine` | Machine model | `vendor`, `board`, `bios` |
| `localip` | First non-loopback IPv4 | `ipv6`, `interface` |
| `packages` | Installed package count | — |

`--list-modules` prints `name: description`; `--list-fields` prints
`module.field: description`. A module is fetched once even when several fields
are referenced. Missing fields disappear with their separator, and unknown
fields are configuration errors. Fields do not apply to `{face}`.

Desktop, terminal, and virtualization detection use environment markers and
system files and are best effort. Bare metal has no `virt` value. `host` remains
the hostname; `machine` reads DMI or device-tree model information.
`packages` and `localip` are opt-in. Package detection reads dpkg, pacman, or apk
databases (in that order for `auto`); RPM is unsupported because it requires a
database library. Large databases may need `--timeout 100ms`.

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
another file. After the user location, smol searches each colon-separated
`$XDG_CONFIG_DIRS` directory (default `/etc/xdg`) for `smol/config.toml`.
The first existing file wins; files are never layered. No config is required.
`--list-config-paths` shows candidates in order and marks existing files with
` (found)`; with `--config`, it shows only that path.

`--print-config` prints the effective configuration as valid TOML, including
flag overrides, so it can be redirected to a new file. Default module option
tables are omitted. A malformed file prints defaults in normal operation;
`--check-config` reports errors. `--print-default-config` prints the commented
template. Examples in [presets/](presets/) can be copied into your config; there
is no runtime preset flag.

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

## Display and module options

All controls are optional. Defaults preserve the original size precision
(KiB/MiB: zero decimals, GiB: one decimal) and duration output (`<1m`, `3d 4h 0m`).

```toml
[display.size]
unit = "iec" # iec | si
digits = -1 # automatic, or 0..6 decimal places
[display.duration]
abbreviation = true # false: "3 days 4 hours 0 minutes"
space-before-unit = false
compact = false # true omits zero units
round = false # true rounds 30 seconds up into the next minute
[display.percent]
digits = 0 # 0..6
width = 10 # 1..100; ASCII bars like [#####-----]
[display.temp]
unit = "c" # c | f | k
digits = 0 # 0..6

[modules.disk]
mount = "/" # free space available to unprivileged users
[modules.time]
format = "15:04" # Go layout; also "15:04:05" or "Mon 15:04"
[modules.battery]
device = "" # first battery; e.g. "BAT1" selects another
[modules.localip]
interface = "" # first non-loopback interface
family = "ipv4" # ipv4 | ipv6
[modules.packages]
manager = "auto" # auto | dpkg | pacman | apk
```

`--debug` writes module timings and failures plus total fetch time to stderr;
stdout stays unchanged. Without `--debug`, normal runs keep stderr empty.
`--debug --animate` is a usage error because animation redraws the terminal.

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
With Go 1.26.10 on Linux amd64, `go build -trimpath` measured 4,038,499 bytes
before the expansion and 4,447,904 bytes after it (+409,405 bytes, about 10.1%).
This includes the opt-in local-IP support and its standard-library `net` import;
no dependencies were added.

The first lint run requires the `golangci-lint` builder and network access.
Use `make lint`, not stock `golangci-lint run`, to load the plugin.
Run `make audit` with `govulncheck` before dependency upgrades or releases.

- `cmd/app/`: process entry point, built as `smol`.
- `internal/app/`: CLI execution with injected filesystem, environment, and writers.
- `internal/config/`: TOML defaults, strict parsing, and validation.
- `internal/modules/`: built-in system-info modules and injected detection hooks.
- `internal/format/`: shared size, duration, percent, and temperature formatting.
- `internal/render/`: format parsing, module snapshots, separators, and color escapes.
- `internal/face/`: faceroll character selection, validation, and discovery.
- `internal/playback/`: synchronous animation with checked writes and terminal cleanup.
- `testdata/`: x86 and ARM Linux fixtures used by deterministic tests.

Caching, `smol init`, GPU modules, and width truncation are not
implemented in the supplied app or this adaptation.

The template's [BSD-2-Clause license](LICENSE) is retained.
