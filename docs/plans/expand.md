# Expansion plan: fastfetch-inspired capabilities

Add per-module format fields, display formatting controls, config discovery, and
a small set of cheap modules inspired by upstream fastfetch. The default format
and its rendered output stay byte-for-byte unchanged; every addition is opt-in.

Analysis baseline: fastfetch `dev` at `cad01fd` (76 modules), smol working tree
after the faceroll integration (post-`4b7ba92`, including `internal/face`,
`internal/playback`, and the `render.Fetch`/`render.Assemble` split). Only
features that read files or call cheap syscalls are in scope.

This plan assumes the faceroll work lands first. Phase 1 changes the same
render/app/config paths, so rebasing it onto a merged faceroll branch is cheaper
than developing both in parallel.

## 0. Ground rules

- **Default output is stable.** `smol` with no arguments and no config must print
  the same line as today. New modules never enter the default format.
- **No subprocesses, network, D-Bus, or dlopen.** Detection reads `/proc`,
  `/sys`, environment variables, or calls `statfs`/`uname`/`getifaddrs`.
- **One latency budget.** `--timeout` remains the single deadline for all
  modules. There are no per-module timeouts in this expansion.
- **No new dependencies.** Keep `go-toml/v2` and `faceroll/go`; new code uses
  the standard library. `syscall` is acceptable for Linux-only calls (`Statfs`,
  `Uname`).
- **Strict configuration.** Unknown modules, fields, keys, and tables remain
  hard errors in `--check-config` and fall back to defaults in normal mode.
- **Failure means absence.** A timed-out, unsupported, or malformed module still
  disappears with its separator; it never prints `n/a`.

## 1. Phase 0 — correctness fixes

Independent, no API changes, can ship first.

| Fix | Change | Test |
| --- | --- | --- |
| Memory fallback | Use `MemFree + Buffers + Cached + SReclaimable - Shmem` when `MemAvailable` is missing, malformed, or greater than `MemTotal`. Preserve valid values from zero through `MemTotal`, inclusive. Clamp fallback arithmetic to `[0, MemTotal]` without unsigned underflow or overflow. | Missing/malformed `MemAvailable`; zero reports all memory used; equality reports zero used; above-total fallback; `Shmem` larger than the fallback sum |
| Shell detection | Prefer the parent process when it is a known shell: read `proc/self/stat` for the ppid, then `proc/<ppid>/comm`; skip `sudo`, `su`, `sh`, `env`, `time`, `strace`, `gdb`. Fall back to `$SHELL`. | Fixtures for `proc/self/stat`, `proc/100/comm`; env fallback cases |
| OS fallback | Try `/etc/os-release`, then `/etc/lsb-release` (`DISTRIB_DESCRIPTION`, else `DISTRIB_ID` + `DISTRIB_RELEASE`), then `/usr/lib/os-release`. | Fixture with only `lsb-release` |
| Memory precision | Keep current KiB/MiB/GiB rules while extracting a shared size formatter for Phase 2. | Existing mem tests unchanged |

Extend `testdata/x86/proc/meminfo` with `SwapTotal`, `SwapFree`, `SReclaimable`,
`Shmem` (existing expected values do not change because `MemAvailable` is
present). Add `testdata/legacy/proc/meminfo` for the fallback path.

## 2. Phase 1 — module registry and typed results

### API

Replace the string-only module contract with typed results:

```go
// internal/modules
type Field struct {
    Name        string
    Description string
}

type Module struct {
    Name        string
    Description string
    Fields      []Field
    Fetch       func(context.Context, Env) (Result, error)
}

type Result struct {
    Text   string            // used by "{module}"
    Fields map[string]string // used by "{module:field}"
}

func Get(name string) (Module, bool)
func All() []Module // sorted system modules, used to build listings
func Names() []string // All names plus "face", sorted
func Known(name string) bool
```

Existing modules move to `Module{Name: "mem", Description: "...", Fields: ..., Fetch: memFetch}`
entries in a package-level registry. `Fetch` signatures change from
`(string, error)` to `(Result, error)`; each existing system module returns
`Result{Text: value}` where it returned a string.

`face` stays special. Its text is produced by `internal/face` from configuration,
not by a system-module fetch, so it remains app-injected through
`render.Options.Lookup`; it has no fields and is listed with a fixed description.
Validation treats it as a module with an empty field set, so `{face:anything}`
is an error. `Names`/`Known` continue to include it so format and style
validation behave as they do now.

### Rendering with fields

`render.Fetch` changes its snapshot type from `map[string]string` to
`map[string]modules.Result`; `Assemble` selects `Text` or `Fields[token.Field]`.
The face path adapts mechanically: app injects `Result{Text: animation.Still}`,
and the animation loop replaces `Result.Text` per frame while leaving other
modules' results untouched. This preserves the existing guarantee that system
information is fetched once and frames only re-format the line.

### Format syntax

Extend `render.Token` with `Field`:

```go
type Token struct {
    Literal string
    Module  string
    Field   string
}
```

- `{mem}` uses `Result.Text`.
- `{mem:percent}` uses `Result.Fields["percent"]`.
- Field names are `[a-z0-9-]+`; more than one `:` in a reference is an error.
- Unknown modules and unknown fields fail `ValidateFormat` with the same style
  of message used today.
- A registered field that is missing at runtime is treated like
  `ErrUnavailable`: that reference and its preceding separator are dropped.
- Each module is fetched once even when the format references several of its
  fields; the renderer selects fields from a single `Result`.
- `{{` and `}}` escaping is unchanged.

This syntax is backward compatible: `mem:percent` is an unknown module today,
so no currently valid format changes meaning.

### Fields exposed by existing modules

| Module | Fields |
| --- | --- |
| `face` | none (app-injected, no fields) |
| `os` | `id`, `version` |
| `kernel` | `arch`, `release`, `name` |
| `uptime` | `days`, `hours`, `minutes`, `seconds` |
| `mem` | `used`, `free`, `available`, `total`, `percent`, `bar` |
| `load` | `percent` (1-minute load / online CPUs × 100) |
| `cpu` | `threads`, `cores`, `freq`, `temp` |
| `host` | none |
| `user` | none |
| `shell` | none |

`percent` and `bar` follow `[display.percent]` (Phase 2). `freq` reports the
maximum advertised frequency from cpufreq sysfs, falling back to `cpu MHz`;
`temp` scans `/sys/class/hwmon` and is unavailable when no sensor exists.

### Environment hooks

Extend `modules.Env` for modules that cannot use `io/fs` alone:

```go
type Env struct {
    FS         fs.FS
    Getenv     func(string) string
    Now        func() time.Time
    Statfs     func(path string) (DiskStats, error)
    Uname      func() (machine string, err error)
    Interfaces func() ([]Interface, error)
    Display    *format.Options // nil means format.Default()
    Options    *Options        // nil means DefaultOptions(); immutable during fetching
}

// All sizes are bytes. Keep free blocks distinct from unprivileged availability.
type DiskStats struct {
    Total     uint64
    Free      uint64
    Available uint64
}

// Module-owned option types; modules never imports the config package.
type Options struct {
    Disk     DiskOptions
    Time     TimeOptions
    Battery  BatteryOptions
    LocalIP  LocalIPOptions
    Packages PackagesOptions
}

func DefaultOptions() Options

type Interface struct {
    Name string
    IPv4 []string
    IPv6 []string
}
```

`cmd/app` populates every hook; tests supply fakes. `app.Runtime` gains matching
optional fields (`Now`, `Statfs`, `Uname`, `Interfaces`) so app-level tests can
inject them; `cmd/app` supplies the real syscall-backed implementations. A nil
`Display` means `format.Default()`, and nil `Options` means `DefaultOptions()`,
so existing module tests keep working without a new literal. A module that needs
a nil hook returns an internal error, which surfaces only under `--debug`.
Production hooks live in `cmd/app`, keeping platform wiring out of `modules`.
The statfs hook converts `Blocks`, `Bfree`, and `Bavail` to byte counts using
the filesystem block size and rejects invalid or overflowing values.

After config resolution and CLI overrides, `internal/app` copies the validated
module configurations into one `modules.Options` value and passes its address
through `Env.Options`. Each fetch reads its own typed options from that value.
This also applies when `render.Options.Lookup` wraps the registry for `face`;
no command mutates registry entries or package-level option state.

### New package

Add `internal/format` with dependency-free option types and helpers used by both
`config` and `modules`:

```go
type Options struct {
    Size     SizeOptions
    Duration DurationOptions
    Percent  PercentOptions
    Temp     TempOptions
}
```

`modules` must not import `config` or `render`; `format` breaks that cycle.

## 3. Phase 2 — display formatting

Add a `[display]` table. Defaults reproduce today's output exactly; every knob
is opt-in.

```toml
[display.size]
unit = "iec"      # iec | si
digits = -1       # -1 = current per-magnitude rule (KiB 0, MiB 0, GiB 1)
[display.duration]
abbreviation = true   # true: "3d 4h"; false: "3 days 4 hours"
space-before-unit = false
compact = false       # true drops zero units ("3d 4m")
round = false         # true rounds >=30s into the next minute (fastfetch behavior)
[display.percent]
digits = 0
width = 10            # bar width; ASCII '#' filled and '-' empty
[display.temp]
unit = "c"            # c | f | k
digits = 0
```

Behavior when a value is omitted comes from `config.Default()`, which is decoded
into before reading the file, so `false` defaults are preserved.

Per-module options use explicit pointer fields rather than a
`map[string]any`:

```go
type Config struct {
    Face    face.Options
    Format  string
    Color   string
    Style   map[string]render.Style
    Display DisplayConfig
    Modules ModulesConfig
}

type ModulesConfig struct {
    Disk     *DiskConfig     `toml:"disk"`
    Time     *TimeConfig     `toml:"time"`
    Battery  *BatteryConfig  `toml:"battery"`
    LocalIP  *LocalIPConfig  `toml:"localip"`
    Packages *PackagesConfig `toml:"packages"`
}
```

This keeps `DisallowUnknownFields` effective: `[modules.diks]` and unknown keys
inside each table both fail with a line number. Adding a module with options
means adding one struct and one validation branch; modules without options need
no config entry and reject a `[modules.<name>]` table automatically.

The per-module config types alias module-owned option structs (for example,
`type DiskConfig = modules.DiskOptions`), with explicit TOML tags on their fields.
`modules.DefaultOptions()` defines mount `/`, time layout `15:04`, battery
device `""`, localip interface `""` and family `ipv4`, and package manager `auto`.
`config.Default()` allocates a fresh, non-nil pointer for every module table,
initialized from those defaults, before TOML decoding. Omitted tables, empty
tables, and omitted keys in partial tables therefore all retain their defaults.
Explicit values (including an empty battery device or interface) remain intact;
invalid explicit values are rejected rather than silently replaced.

For `--print-config`, compare each effective module value against
`modules.DefaultOptions()` and omit equal tables regardless of pointer identity
or whether the input file contained an empty table. App wiring copies these
values into `Env.Options`; it never passes `config.Config` to `modules`.

`formatKiB` and `formatUptime` move into `internal/format` and accept options.
The default path keeps the existing formatting, including the `<1m` cutoff and
always-printed lower units.

## 4. Phase 3 — discovery and diagnostics

### Config search

Resolution order when `--config` is not given:

1. `$XDG_CONFIG_HOME/smol/config.toml` (or `$HOME/.config/smol/config.toml` when
   `XDG_CONFIG_HOME` is unset)
2. each directory in `$XDG_CONFIG_DIRS` (colon-separated, default `/etc/xdg`),
   as `<dir>/smol/config.toml`

The first existing file wins; there is no layering. Add:

- `config.Search(getenv, exists func(string) bool) []Candidate` so tests can
  cover the order without touching disk.
- `--list-config-paths`: print candidates in order and mark every existing file
  with ` (found)`. Only the first existing file is loaded.
- `--print-config`: print the effective configuration as valid TOML in a fixed
  key order, including flag overrides and omitting module tables that are still
  at their defaults. A malformed config prints the defaults; use
  `--check-config` for the error. Users redirect the output to create a config;
  no file-writing code. Handle this after config resolution and flag overrides,
  unlike the early `--print-default-config` branch.
- `--print-default-config`, `--check-config` keep their current output.

With `--config`, `--list-config-paths` prints that one path and whether it
exists, instead of the search list.

Explicit `--config` continues to require an existing file. A malformed file
still falls back to defaults in normal mode.

### Diagnostics

Add `--debug` (boolean). It writes only to stderr and never changes stdout:

```text
smol: cpu 0.3ms ok
smol: mem 0.5ms ok
smol: packages 22.1ms timeout
smol: total 20.2ms
```

Include dropped/unknown modules and, when `--check-config` is used, the resolved
config path. `--debug` is mutually exclusive with `--animate` (usage error):
the animation clears and redraws the line, which would garble the debug stream.
Tests assert stderr is empty without `--debug`.

### Module listing

`--list-modules` prints `name: description` (fastfetch style) and updates the
README. This is a documented output change; scripts that need bare names can
still parse before the first `:`. The listing includes `face` with a fixed
description. Fields are listed by a separate `--list-fields`, which prints
`module.field: description` for system modules. Neither command changes stdout
formatting for normal runs.

## 5. Phase 4 — new modules

All modules are opt-in: they appear only when referenced by the format. Style
defaults extend `config.Default()` with one palette entry per new module.

### Batch A — files and environment only

| Module | Default text | Fields | Config | Sources |
| --- | --- | --- | --- | --- |
| `swap` | `used / total` | `used`, `free`, `total`, `percent`, `bar` | none | `/proc/meminfo` (`SwapTotal`, `SwapFree`) |
| `disk` | `used / total` | `used`, `free`, `available`, `total`, `percent`, `bar` | `[modules.disk] mount = "/"` | `Env.Statfs` |
| `battery` | `82%` | `percent`, `status` | `[modules.battery] device = ""` (first battery) | `/sys/class/power_supply/*` |
| `terminal` | `kitty` | none | none | `TERM_PROGRAM`, `KITTY_PID`, `WEZTERM_EXECUTABLE`, `WT_SESSION`, `SSH_TTY`/`SSH_CONNECTION`, `TERM` |
| `desktop` | `GNOME` | `wm`, `session` | none | `XDG_CURRENT_DESKTOP`, `DESKTOP_SESSION`, `WAYLAND_DISPLAY`, `HYPRLAND_INSTANCE_SIGNATURE`, `SWAYSOCK` |
| `virt` | `docker` | none | none | `/.dockerenv`, `/run/.containerenv`, `/proc/1/cgroup`, DMI product name, `hypervisor` flag in `/proc/cpuinfo`, `WSL_DISTRO_NAME` |
| `time` | `15:04` | none | `[modules.time] format = "15:04"` (Go layout) | `Env.Now` |
| `processes` | `312` | none | none | count `/proc/<digits>` |
| `init` | `systemd` | none | none | `proc/1/comm` |

Notes:

- `desktop` is best effort. It never starts a process or talks to D-Bus; when
  the environment is ambiguous it returns `ErrUnavailable`.
- `virt` stays conservative: only well-known markers and DMI strings. Bare metal
  returns `ErrUnavailable`.
- `battery` uses the first battery by default and reports `charging`,
  `discharging`, `full`, or `unknown` from its `status`; `device` selects
  another entry under `/sys/class/power_supply`.
- `disk` uses `used = Total - Free`, where `Free` includes reserved free blocks;
  `available` is separately taken from `Available` (unprivileged availability).
  `percent` and `bar` use `used / (used + available)`, matching the capacity
  visible to an unprivileged user. A zero denominator omits those fields while
  retaining valid size fields. Reject inconsistent stats (`Free > Total` or
  `Available > Free`) instead of allowing unsigned subtraction to wrap.
- `time` uses Go layouts rather than strftime; document `15:04:05` and `Mon 15:04`.

### Batch B — syscalls, sysfs scans, and file databases

| Module | Default text | Fields | Config | Notes |
| --- | --- | --- | --- | --- |
| `machine` | DMI product name | `vendor`, `board`, `bios` | none | `/sys/class/dmi/id/*`, device-tree fallback; trim NULs; reject OEM placeholders |
| `localip` | first non-loopback IPv4 | `ipv6`, `interface` | `[modules.localip] interface = ""`, `family = "ipv4"` | `Env.Interfaces`; opt-in because importing `net` grows the binary and addresses are sensitive |
| `packages` | count | none | `[modules.packages] manager = "auto"` | count `Status: install ok installed` in `/var/lib/dpkg/status`, count `/var/lib/pacman/local/*`, or parse `/lib/apk/db/installed`; no subprocess |

`machine` deserves a separate name: `host` remains the hostname, avoiding a
breaking change. Placeholder filtering copies fastfetch's list ("To Be Filled By
O.E.M.", "Default string", "System Product Name", "None", "Unknown", "N/A", …).

`packages` is the only module that can plausibly exceed the default budget on a
large system. It is opt-in, documented with `--timeout 100ms`, and uses streaming
line counts rather than loading the database into memory. `manager = "auto"`
tries dpkg, then pacman, then apk, and uses the first database that exists. RPM
is not supported: it needs a database library, which would violate the no-dlopen
rule; `manager = "rpm"` is a validation error with a message explaining why.

### Additional fields requiring no new module

- `{kernel:arch}` via `Env.Uname` (machine string) with `runtime.GOARCH` fallback.
- `{cpu:cores}` from `cpu cores`, `{cpu:freq}` from cpufreq sysfs, `{cpu:temp}`
  from `/sys/class/hwmon` (maximum CPU package/core sensor; unavailable when no
  sensor exists).
- `{uptime:days|hours|minutes|seconds}` and `{load:percent}`.
- `{os:id}`, `{os:version}`, `{kernel:release}`.

## 6. Phase 5 — presets and documentation

- Add `presets/banner.toml`, `presets/prompt.toml`, `presets/server.toml`, and
  `presets/minimal.toml`. They are examples to copy, not a runtime `--preset`
  flag (avoids config precedence complexity).
- Update `README.md` with the module/field table, `[display]`, `[modules.*]`,
  config search order, `--debug`, `--list-config-paths`, `--print-config`, and
  `--list-fields`. Remove `battery`/`packages` from the "not implemented" list
  and update the "nine built-in modules" wording.
- Update `sample.env` with new environment inputs: `XDG_CONFIG_DIRS`,
  `TERM_PROGRAM`, `KITTY_PID`, `WEZTERM_EXECUTABLE`, `WT_SESSION`, `SSH_TTY`,
  `SSH_CONNECTION`, `XDG_CURRENT_DESKTOP`, `DESKTOP_SESSION`, `WAYLAND_DISPLAY`,
  `HYPRLAND_INSTANCE_SIGNATURE`, `SWAYSOCK`, `WSL_DISTRO_NAME`.
- Extend the default-config template with commented examples for the new tables
  while keeping the existing `[face]` example and animation notes.
- Keep the faceroll/face sections intact; document that fields do not apply to
  `{face}` and that `--debug` cannot be combined with `--animate`.
- Note explicitly in the README that desktop/terminal/virt detection is
  environment-based and best effort, and that packages/localip are opt-in.

## 7. Testing

- **Modules:** table tests with fixture trees for every new module (`swap`,
  `battery`, `machine`, `init`, `processes`, `virt`, `packages`, `os` fallback,
  `shell` parent chain). Fake hooks for `disk`, `localip`, `time`, `uname`.
- **Memory boundaries:** preserve zero and total `MemAvailable`; fallback for
  missing, malformed, and above-total values; clamp fallback arithmetic safely.
- **Disk accounting:** fake stats with reserved blocks (`Total=100`, `Free=20`,
  `Available=15`) report used 80, free 20, available 15, and percent 84 with
  default precision. Cover zero capacity and inconsistent stats as well.
- **Fields:** every registered field has a test; a missing field drops that
  reference and its separator, matching `ErrUnavailable`.
- **Format parser:** `{mem}`, `{mem:percent}`, `{mem:percent:extra}` (error),
  `{mem:}`, unknown field, `{{`/`}}`, control characters.
- **Display options:** default output equals the current rendering; explicit
  IEC/SI, digits, abbreviation, compact, rounding, percent bar/number.
- **Config:** XDG search order, `--list-config-paths` markers, `--print-config`
  round-trip (print then parse), unknown module table, unknown key.
- **Module options:** omitted and empty tables retain every module default;
  partial localip config retains `family = "ipv4"`; explicit empty selectors
  remain valid; default tables are omitted when printing. App tests verify that
  custom mount, time layout, battery device, interface/family, and package manager
  reach their fetches, and separate runs do not share mutable option state.
- **App:** `--debug` stderr lines, empty stderr without it, timeout reporting,
  `--list-modules` descriptions, `--list-fields`, exit codes unchanged.
- **Face compatibility:** static and animated `{face}` output is unchanged by
  typed results; animation still reuses one fetch; `--debug --animate` exits 2.
- **Golden output:** a default run with no config still produces the current
  line, including trailing newline and separator behavior.
- Run `make fix` and `make check` after each phase. Run `make audit` before the
  final merge. Keep the 80% internal coverage floor; new modules need negative
  and unavailable paths, not just happy paths.

## 8. Non-goals

- Logo/ASCII art, multi-line output, title/separator/break modules, keys/icons.
- Watch/dynamic-refresh mode, in-process or on-disk result caching.
- Sampling modules: CPU usage, network/disk throughput, top processes.
- Network modules: public IP, weather, DNS, Wi-Fi.
- D-Bus/MPRIS: media, players, Bluetooth, sound, display/monitor, input devices,
  wallpaper, themes, icons, fonts, cursor.
- GPU/OpenGL/Vulkan/OpenCL and any dlopen-based detection.
- DE/WM *versions* and package managers that require spawning commands.
- `--json` output. Typed results make it easy, but it broadens the interface;
  revisit after the module work settles.
- Multi-platform support. Linux-only remains explicit.
- Per-module timeouts. `--timeout` governs all modules; `packages` users raise it.
- i18n, config schema generation, shell completions.

## 9. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Default line changes unintentionally | Golden test of `smol` output with defaults; defaults for every `[display]` key reproduce current formatting |
| `{module:field}` breaks existing configs | Syntax is new; no previously valid token is reinterpreted; validation errors name the field and module |
| Registry refactor churns all tests | One mechanical commit: registry + `Result{Text}`; behavior tests unchanged; fields added afterwards |
| Typed results disturb face/playback | Adapt the existing one-fetch `Fetch`/`Assemble` split; add static and animated face regression tests |
| `--list-modules` output change | Document it; keep `name: description` on one line; `--list-fields` covers field discovery |
| `net` import grows the binary | Accept for opt-in `localip`; measure `dist/smol` before/after and record the delta in the README |
| `statfs` blocks on a dead network mount | The renderer already returns at the deadline; the process exits after the write, so a blocked goroutine cannot hang the prompt |
| Environment heuristics misreport desktop/terminal | Best-effort only; return `ErrUnavailable` when markers conflict; no subprocess fallback |
| `packages` exceeds 20 ms | Opt-in, streaming, documented `--timeout` guidance, `--debug` shows the timing |
| Env hook growth leaks platform code into modules | Hooks are supplied by `cmd/app`; `modules` imports only `format` and stdlib |

## 10. Implementation order

1. **Correctness** — mem fallback, shell parent chain, lsb-release, fixtures.
2. **Foundation** — `internal/format`, registry, `Result`, `{module:field}`,
   validation, field tests.
3. **Display** — size/duration/percent/temp options and their golden tests.
4. **Core fields** — mem, uptime, load, cpu, kernel, os fields.
5. **Discovery** — XDG search, `--list-config-paths`, `--print-config`,
   `--debug`, `--list-modules` descriptions, `--list-fields`.
6. **Batch A modules** — swap, disk, battery, terminal, desktop, virt, time,
   processes, init.
7. **Batch B modules** — machine, localip, packages; cpu temp; kernel arch.
8. **Docs** — presets, README, `sample.env`, default config; `make audit` and a
   terminal plus redirected-output smoke test.

Each step ends with `make fix && make check`. Steps 2–4 should land before any
module that relies on fields or display options, so new modules never invent
their own formatting helpers.

## 11. Comparison with fullfetch and dawnfetch

Source review baseline: fullfetch `3f47cf0` (version 3.2.0) and dawnfetch
`bb540c8` (version 0.1.6). The expansion column describes planned behavior,
not features already implemented in smol. No common performance benchmark was
run; a module deadline is not an end-to-end startup measurement.

| Capability | smol now | smol after this plan | fullfetch | dawnfetch |
| --- | --- | --- | --- | --- |
| Output | One configurable line; optional animated face | Same output model, with selectable fields | Multi-line summary beside distro/custom ASCII art | Multi-line themed summary; ASCII or image logo; interactive theme preview |
| Platforms | Linux | Linux | Linux, Windows, macOS | Linux, Windows, macOS |
| Swap, disk, battery, local IP, packages | Not implemented | Opt-in modules | Implemented | Implemented; package collector excluded on Windows |
| Desktop and terminal | Shell only | Environment-based desktop and terminal detection | No dedicated desktop/terminal entries in the action registry | Desktop, WM, terminal, resolution, and visual settings collectors |
| Hardware detail | CPU model and logical thread count | Core count, frequency, CPU temperature, machine/board/BIOS fields | CPU frequency, GPU, motherboard, RAM model | CPU/GPU details and temperature summaries in full mode |
| Configuration | Strict TOML; CLI format/color overrides; config check | Typed module/display options, field discovery, effective TOML export | JSON field/order/color/art schemes; generation and migration commands | JSON user/theme settings; theme selection and diagnostics commands |
| Collection policy | Fetch referenced modules concurrently; shared 20 ms default deadline | Preserve shared deadline and silent omission | Selected display functions run sequentially; package counters run concurrently; no shared collection deadline | Collectors run concurrently and wait for all; command-specific deadlines and fast/full modes |
| External probes | No subprocesses or network probes | Same constraint; interface enumeration for local IP | GPU/RAM/RPM probes invoke commands; local IP resolves the hostname | External probes use a command runner with individual timeouts |

Evidence: fullfetch's [dispatch and configuration loader](https://github.com/Buct0r/fullfetch/blob/3f47cf0fd62777bc808839951a61e23dddd12fc0/loadconfig.go),
[collectors and renderer](https://github.com/Buct0r/fullfetch/blob/3f47cf0fd62777bc808839951a61e23dddd12fc0/functions.go),
and [dependencies](https://github.com/Buct0r/fullfetch/blob/3f47cf0fd62777bc808839951a61e23dddd12fc0/go.mod);
dawnfetch's [CLI](https://github.com/almightynan/dawnfetch/blob/bb540c8aee6169a98d9a7c2e45be56dedfefc8b6/internal/dawnfetch/cli/run.go),
[help](https://github.com/almightynan/dawnfetch/blob/bb540c8aee6169a98d9a7c2e45be56dedfefc8b6/internal/dawnfetch/cli/help.go),
[collector registry](https://github.com/almightynan/dawnfetch/blob/bb540c8aee6169a98d9a7c2e45be56dedfefc8b6/internal/dawnfetch/system/collect_registry.go),
and [command runner](https://github.com/almightynan/dawnfetch/blob/bb540c8aee6169a98d9a7c2e45be56dedfefc8b6/internal/dawnfetch/system/exec.go).

The useful overlap is the cheap system data already covered by this expansion.
Preserve smol's prompt-oriented output, shared deadline, strict config, and
failure-as-absence behavior. Fullfetch's multiple mounts/batteries and combined
package-manager counts, and dawnfetch's interactive themes/images, are broader
product choices rather than prerequisites for this plan. Boot time and locale
are possible cheap follow-ups from fullfetch, but remain outside this expansion.

## Source references

- Memory: `src/detection/memory/memory_linux.c` (fallback, ZFS ARC).
- Linux [available-memory calculation](https://github.com/torvalds/linux/blob/master/mm/show_mem.c)
  explicitly permits zero; do not replace it with the legacy fallback.
- Linux [`statfs(2)`](https://man7.org/linux/man-pages/man2/statfs.2.html)
  distinguishes all free blocks (`f_bfree`) from unprivileged availability
  (`f_bavail`).
- Uptime: `src/common/impl/duration.c` (rounding, units), `src/modules/uptime`.
- Shell: `src/detection/terminalshell/terminalshell_linux.c` (parent chain).
- OS: `src/detection/os/os_linux.c` (os-release, lsb-release order).
- Host/SMBIOS: `src/detection/host/host_linux.c`, `src/common/impl/smbios.c`.
- Display options: `src/options/display.c`, `src/common/impl/duration.c`.
- Packages: `src/detection/packages/packages_linux.c` (file-based dpkg count).
- CLI/config: `src/fastfetch.c` (`--gen-config`, `--list-*`, `--structure`);
  smol uses `--print-config` and shell redirection instead of writing files.
