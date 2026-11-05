# Faceroll integration plan

Add an opt-in `{face}` module with static expressions and an explicit animated
terminal mode. Existing `smol` output stays unchanged until the format includes
`{face}`.

Faceroll supplies seeded characters, presets, still expressions, and timed
animation frames. Its Go module has no third-party dependencies.
[Faceroll Go documentation](https://github.com/rigerc/faceroll/blob/main/go/README.md).

## 1. User-facing behavior

```sh
# Static face
smol --format '{face} {host} · {mem}'

# Named character and expression
smol --format '{face} {host}' --face-preset kitty --face-state happy

# Repeatable generated character
smol --format '{face} {host}' --face-seed my-machine

# Animate until interrupted
smol --format '{face} {host} · {mem}' --animate

# Animate for a fixed duration
smol --format '{face} {host}' --animate --duration 5s
```

Default expression: `idle`. Without a preset or explicit seed, generate a
deterministic character using seed `"smol"`. Repeated `{face}` references show
the same character and synchronized frame.

## 2. Configuration and overrides

```toml
format = "{face} {user}@{host} · {mem}"

[face]
preset = "kitty"
state = "idle"
# Instead of preset:
# seed = "my-machine"

[style]
face = { fg = "cyan", bold = true }
```

Validate preset and state identifiers through faceroll. Preserve an explicitly
empty seed as a valid deterministic seed.

Explicit preset and seed settings are mutually exclusive. A CLI preset replaces
a configured seed; a CLI seed replaces a configured preset. Passing both CLI
selectors is an error.

Keep animation CLI-only so an ordinary invocation cannot unexpectedly become a
long-running process.

## 3. Discovery commands

- Include `face` in `--list-modules`.
- Add `--list-face-presets`.
- Add `--list-face-states`.
- Extend help and `--print-default-config` with face examples.

Derive available presets and states from faceroll's tables rather than maintaining
duplicate lists. Treat those exported tables as read-only, as required by its
documentation.

## 4. Dependency and character creation

Pin `github.com/rigerc/faceroll/go` to a verified Go release or commit-derived
pseudo-version.

Add a small `internal/face/` package that validates options and builds one
character per invocation. Use `CreatePresetCharacter` or seeded `Roll`, returning
the selected animation to the application.

Recognize `face` during format/style validation and supply its configured value
through an injected module lookup. Keep configuration dependencies out of the
system-information modules.

## 5. Separate fetching from formatting

Refactor rendering into two operations:

- Fetch referenced system modules once, retaining the shared timeout.
- Assemble a line from tokens, fetched values, and the current face text.

Static mode uses the selected animation's `Still` value. Animated mode substitutes
frame text without rereading `/proc`, rebuilding the character, or spawning module
goroutines per frame.

Apply existing sanitization, separator handling, colors, and shell escaping to
every face value. Never parse generated face text as format syntax.

## 6. Animation with checked writes

Use faceroll's frames and their individual millisecond durations. Implement a
synchronous, context-aware playback loop in smol so write failures propagate and
shutdown completes before returning.

Faceroll's terminal player currently discards write errors, and its player's
`Stop` signals cancellation without waiting for its goroutine. Owning the small
output loop gives smol control over both behaviors.
[Terminal implementation](https://github.com/rigerc/faceroll/blob/main/go/terminal.go),
[player implementation](https://github.com/rigerc/faceroll/blob/main/go/player.go).

During playback:

- Draw the first frame immediately.
- Clear and redraw the complete line for subsequent frames.
- Hide the cursor while active.
- On completion, leave the final frame visible, restore the cursor, and append
  one newline.
- Stop immediately on output failure; attempt terminal restoration before
  returning the error.
- Print once and exit for a single-frame expression.

## 7. Terminal and cancellation rules

| Situation | Behavior |
| --- | --- |
| Interactive terminal with `--animate` | Play animation |
| Redirected output or `TERM=dumb` | Print one static line and exit |
| `--animate` without `{face}` | Usage error |
| `--animate` with `--shell bash/zsh/fish` | Usage error; prompt embedding stays static |
| Positive `--duration` | Stop after that duration |
| No duration | Continue until interrupted |
| `--duration` without `--animate` | Usage error |

Detect terminal width at the process boundary. Animate only when the longest
rendered frame fits without wrapping; otherwise print a static line. Recheck
after terminal resize and stop playback cleanly if it no longer fits.

Handle SIGINT and SIGTERM at the composition root through context cancellation.
Return conventional signal exit codes, `130` and `143`. Preserve existing success,
configuration-error, and usage-error codes.

## 8. Configuration fallback behavior

Malformed face configuration follows smol's existing normal-mode fallback to
defaults. `--check-config` reports the actual error.

Invalid explicit CLI options return exit code `2`. Validate configuration even
when the chosen format omits `{face}`, but only build a character when it is needed.

## 9. Tests and documentation

Cover deterministic generation, presets and states, selector precedence, invalid
configuration, repeated faces, special characters, styling, and shell escaping.

Use injected timers and terminal capabilities to test frame timing, duration
expiry, cancellation, shrinking frames, width fallback, resize handling, output
failures, and cursor restoration without real sleeps.

Update the README, default-config example, and project instructions. Document that
faceroll's asciimoji include Unicode; strict printable-ASCII filtering remains a
separate enhancement.

Finish with `make fix`, `make check`, `make audit`, and interactive-terminal plus
redirected-output smoke checks.

## Implementation order

1. Static face support.
2. Reusable rendering.
3. Animation and terminal cleanup.
4. Regression tests and documentation.
