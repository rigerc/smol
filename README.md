<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="assets/logo-light.png">
    <img src="assets/logo-dark.png" alt="smol" width="360">
  </picture>
</h1>

<p align="center"><strong>One line of Linux system information for login banners, terminal greetings, and shell prompts.</strong></p>

<p align="center">
  <a href="https://github.com/rigerc/smol/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/rigerc/smol/ci.yml?branch=main&style=flat-square" alt="CI"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.26.10-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26.10"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-BSD--2--Clause-blue?style=flat-square" alt="BSD-2-Clause license"></a>
</p>

<p align="center"><a href="#install">Install</a> · <a href="#quick-start">Quick start</a> · <a href="#configuration">Configuration</a> · <a href="#cli-reference">CLI reference</a></p>

<p align="center">
  <img src="demos/vhs/demo.gif" alt="Terminal recording of smol displaying Linux system information with an optional character" width="800">
</p>

<p align="center"><sub>Recorded with <a href="https://github.com/charmbracelet/vhs">VHS</a>. <a href="demos/vhs/demo.tape">View the recording script</a>.</sub></p>

Run `smol` to see your operating system, kernel, uptime, and memory usage without a multi-line system report.

```text
$ smol
Ubuntu 24.04.1 LTS · 6.8.0-48-generic · 3d 4h 4m · 4.6 GiB / 15.6 GiB
```

Values depend on your machine. The default output is one line, with a trailing newline. smol reads system files and environment data locally; it does not spawn system commands or make network requests.

## Install

**Requirements:** Linux and Go **1.26.10**. Building from the repository also requires GNU Make.

### With Go

```sh
go install github.com/rigerc/smol/cmd/app@latest
```

The current Go entry point is `cmd/app`, so `go install` names the executable `app` rather than `smol`. Rename it in your Go binary directory:

```sh
install_dir="$(go env GOBIN)"
if [ -z "$install_dir" ]; then install_dir="$(go env GOPATH)/bin"; fi
mv "$install_dir/app" "$install_dir/smol"
"$install_dir/smol"
```

Add that directory to your `PATH` if you want to invoke `smol` from anywhere. This installs from Go module source, not a prebuilt binary.

### From the repository

```sh
git clone https://github.com/rigerc/smol.git
cd smol
make build
./dist/smol
```

`make build` produces `dist/smol`. `make run` runs the command directly from source.

## Quick start

The default format works without a configuration file. Override it with `--format` to choose the modules and fields you want:

```sh
# A small server status line
smol --format '{host} · {uptime} · {load} · {mem}'

# Show individual fields
smol --format '{user}@{host} · {mem:percent} · {disk:available}'

# Enable Nerd Font icons for a compatible terminal
smol --icons auto --icon-set nerd-font

# Pick a fixed character and expression
smol --format '{face} {host} · {mem}' --face-preset kitty --face-state happy
```

If you built from source without adding `dist/` to your `PATH`, replace `smol` in these examples with `./dist/smol`.

### Available modules

| Information | Modules |
| --- | --- |
| OS and kernel | `{os}`, `{kernel}` |
| CPU and memory | `{cpu}`, `{load}`, `{mem}`, `{swap}` |
| Storage and power | `{disk}`, `{battery}` |
| User and terminal | `{user}`, `{host}`, `{shell}`, `{terminal}`, `{desktop}` |
| Machine and environment | `{machine}`, `{virt}`, `{init}`, `{processes}` |
| Network, packages, and time | `{localip}`, `{packages}`, `{uptime}`, `{time}` |
| Optional text character | `{face}` |

Use `{module:field}` to select a field, such as `{cpu:threads}`, `{mem:percent}`, or `{disk:available}`. Use `{{` and `}}` for literal braces.

```sh
smol --list-modules
smol --list-fields
smol --list-face-presets
smol --list-face-states
```

Unavailable values are omitted along with their separators. Modules are fetched concurrently with a shared **20 ms** default deadline. For slower sources such as package databases, use `--timeout 100ms`. The `packages` module supports dpkg, pacman, and apk databases; RPM is not supported.

## Configuration

### Interactive setup

```sh
smol init
```

Choose a preset, modules, fields, order, icons, colors, an optional character, and shell instructions. The wizard previews choices as you make them, shows the TOML before saving, and requires explicit confirmation. It never edits your shell startup files.

Use `smol init --config ./smol.toml` to write to a different location. The default is `$XDG_CONFIG_HOME/smol/config.toml`, or `$HOME/.config/smol/config.toml` when `XDG_CONFIG_HOME` is unset.

### Edit TOML directly

```toml
format = "{face} {user}@{host} · {os} · {mem}"
color = "auto"

[face]
preset = "kitty"
state = "idle"

[icons]
mode = "auto"
set = "nerd-font"

[style]
face = { fg = "cyan", bold = true }
mem = { fg = "yellow" }
```

Icons are disabled by default. Select `nerd-font` with a compatible Nerd Font, or set `[icons] set = "ascii"`. Icon visibility can be `auto`, `always`, or `never`. For example, `smol --icons always` includes icons in redirected output.

Styles support foreground/background colors, bold, dim, italic, underline, and reverse attributes. Colors include terminal names, `bright-` variants, `index:0` through `index:255`, and hex values such as `#a6e3a1`.

By default, smol searches your user config first, then `$XDG_CONFIG_DIRS` (default `/etc/xdg`); the first existing file wins. Select another file with `--config PATH`. See the sample files in [`presets/`](presets/) and documented [environment inputs](sample.env).

```sh
smol --print-default-config  # Commented template
smol --check-config          # Validate your configuration
smol --print-config          # Effective TOML, including flag overrides
smol --list-config-paths     # See the lookup order
```

If a config file is malformed, normal output uses defaults. Run `--check-config` for a diagnostic. `--format`, `--color`, `--icons`, and `--icon-set` override file settings for that invocation.

### Terminal animation and prompts

A `{face}` token uses [faceroll](https://github.com/rigerc/faceroll/tree/main/go). The character stays consistent for a given seed, and `[face] random = true` selects a fresh character per invocation.

```sh
smol --format '{face} {host}' --animate --duration 5s
smol --format '{user}@{host} · {mem}' --color always --shell bash
```

Animation requires an interactive terminal and cannot be combined with `--shell bash`, `zsh`, or `fish`. Piped output shows a static frame. Shell options only adapt color escape sequences; you add the command to your prompt yourself.

## CLI reference

| Command or flag | Effect |
| --- | --- |
| `smol` | Print the configured one-line summary |
| `smol init` | Configure smol interactively |
| `--format '...'` | Choose modules, fields, and separators |
| `--config PATH` | Use a specific TOML config |
| `--color auto / always / never` | Control terminal colors |
| `--icons auto / always / never` | Control icon visibility |
| `--icon-set nerd-font / ascii` | Choose the icon glyph set |
| `--shell none / bash / zsh / fish` | Escape color codes for a shell prompt |
| `--face-preset`, `--face-seed`, `--face-state` | Select an optional character |
| `--animate`, `--duration 5s` | Animate a face on a terminal |
| `--timeout 100ms` | Increase the shared module deadline |
| `--debug` | Print module timings and failures to stderr |
| `--list-modules`, `--list-fields` | Discover placeholders and selectors |
| `--check-config`, `--print-config` | Validate or inspect config |

See `smol --help` for the complete options, including face discovery and configuration path inspection.

## Development

```sh
make fix
make check
```

`make check` runs formatting checks, vet, the repository's custom golangci-lint, race-enabled tests, an **80% internal coverage gate**, and a build. The first lint run needs the pinned custom linter builder. See the [Makefile](Makefile) for additional targets.

Issues and pull requests are welcome.

## License

[BSD-2-Clause](LICENSE).
