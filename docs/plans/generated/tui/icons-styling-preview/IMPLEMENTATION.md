# Implementation and validation

Implemented against Go 1.26.10, tuiweave v0.3.0, Bubble Tea v2.1.0, and Lip Gloss
v2.0.6. go-nf is pinned to v0.0.0-20261103002006-052bb0b267f6 (Nerd Fonts 3.5.1).
No fontconfig subprocess, font directory scan, or network operation is added to
normal CLI output or setup previews.

## Files and ownership

- `internal/icons`: validated icon policy, ASCII/Nerd Font module mappings, OS
  glyph resolution, and literal overrides.
- `internal/render`: richer SGR styles and decoration after token availability
  checks, before shell escaping. Existing animation measurement includes icons.
- `internal/config`, `internal/app`: TOML defaults/validation/printing and CLI
  overrides; existing ordinary defaults stay undecorated.
- `internal/setup/preview.go`: independent tentative drafts, shared module
  snapshots, and generation-based invalidation.
- `internal/setup/wizard.go`: one inline renderer, immutable questions from the
  workflow, serialized scrollback output, sizing, and clean basic-mode handoff.
- `internal/setup/styles.go`: icon policy/set and per-module style editors.
- `internal/brand`: FIGlet Standard lettering and sampled theme palettes.
- `assets/logo-dark.png`, `assets/logo-light.png`: exact user-supplied files.
  SHA-256 values match the original attachments; the README selects by theme.

## Validation performed

- `make check`: passes, including custom anti-slop lint (0 issues), go vet,
  race-enabled tests, 88.7% aggregate internal statement coverage, and binary build.
- `make audit`: no vulnerabilities found.
- `go mod tidy -diff`: no changes; `go mod verify`: all modules verified.
- Strict feature-contract build: 0 errors, 0 warnings; offline viewer and semantic
  storyboard generated alongside these documents.
- PTY tests: rich/basic save, rich cancellation, SIGTERM, narrow fallback, clean
  stdout, restoration of terminal state, and no alternate screen. Rich saving
  runs at 80 columns by 24 rows and checks the completion diagnostic.
- Preview tests: draft isolation, tentative colors/icons, stale read rejection,
  cache invalidation, empty values, resizing, clipping, inspection coordinates,
  and rich style editing. Updated goldens reviewed for Standard lettering and
  the additional menu options; persistent preview has 40/80-column snapshots.
- Live CLI smoke check: ASCII icons with color disabled; printed default TOML
  validates with `--check-config`. Icon-inclusive animation width fallback is
  covered by an app test.

The additional tuiweave kit audit (`--strict --verify`) reports 0 blocking
findings and 1 advisory about pre-existing @latest examples in local skill/docs
files. Its module verification, build, vet, and tests pass. The advisory is not
emitted by make; no check or warning was suppressed to hide it.

Font-dependent Nerd Font glyph appearance remains a terminal compatibility
limitation; tests validate emitted text, escaping, and cell-width handling, not
which font a terminal actually selects. Basic mode is deliberately append-only.
