# Inline setup implementation plan

Status: implemented; verification evidence is recorded below.
Prepared against the working-tree v4 design on 2026-11-05 using
`charmed-ecosystem`, `charmed-bubbletea`, and `tuiweave-app-builder` in greenfield
mode. smol has an existing CLI, but no interactive UI to migrate.

## Authority and scope

[contracts.json](contracts.json) owns data, commands and validation;
[machine.json](machine.json) owns transitions; [screens.json](screens.json) owns
screen structure. [SPEC.md](SPEC.md) and [DECISIONS.md](DECISIONS.md) explain the
approved behavior. Implement their current working-tree contents, including the
field-selection step. Keep generated artifacts and existing user edits intact.
This document adds implementation sequencing; it does not amend the design.

Deliver `smol init` as an explicit inline command. Preserve ordinary rendering,
configuration discovery, display/module options, deterministic faces and
animation. No shell-file edits, shell execution, installation, automatic
first-run setup, animation editor or system configuration writes.

The subsequent user instruction “Implement” approved the scoped tuiweave stack
exception described by this plan. SPEC now prohibits unrelated dependencies;
the selected UI dependencies are pinned.

## Verified dependency baseline

Selected `github.com/ishan5ain/tuiweave v0.3.0`, resolved by `go list -m -json`
to commit `c05ccf548162f861736844f729c18104949c90e5`. Read that revision's
[agent contract](https://github.com/ishan5ain/tuiweave/blob/c05ccf548162f861736844f729c18104949c90e5/AGENT-CONTRACT.md),
[compatibility policy](https://github.com/ishan5ain/tuiweave/blob/c05ccf548162f861736844f729c18104949c90e5/COMPATIBILITY.md),
[catalog](https://github.com/ishan5ain/tuiweave/blob/c05ccf548162f861736844f729c18104949c90e5/AGENT-CATALOG.md),
and AGENTS recipes for layout, theme, menu, focus, text editing, spinner,
inspection and snapshots. Its selected runtime is Bubble Tea v2.1.0 and Lip
Gloss v2.0.6; the library minimum is Go 1.26.0. smol retains its own Go
1.26.10 minimum. Verify the final selected graph on that exact smol version
with automatic toolchain switching disabled.

Use tuiweave components instead of adding Bubbles or Huh alongside them.
Add only the direct modules actually imported, with their pinned versions;
inspect the resulting transitive graph and binary-size change. No local
`replace`, library fork, theme picker, or speculative agentic packages.

Closest runnable reference:
[examples/frame/main.go](https://github.com/ishan5ain/tuiweave/blob/c05ccf548162f861736844f729c18104949c90e5/examples/frame/main.go)
for menu results, focus and component updates. Borrow its wiring, not its
fullscreen dashboard, borders or terminal-height layout. Use the text editing
recipe for a wrapping seed editor and the spinner recipe for transient work.
There is no verified inline wizard example; the first slice proves that
adaptation before investing in the rest of the UI.

## Architecture and ownership

Keep process resources in `cmd/app`, dispatch in `internal/app`, and introduce
`internal/setup` for wizard policy, terminal adapters and persistence. Prefer
small concrete structs and narrow interfaces at side-effect boundaries; avoid
a general-purpose form framework or a runtime JSON interpreter.

| Owner | Responsibility |
| --- | --- |
| `cmd/app` | stdin/stderr terminal detection, startup cwd, entropy, signals, terminal resources and real filesystem operations |
| `internal/app` | dispatch `args[0] == "init"` before root parsing; ordinary rendering and effective face overrides |
| `internal/setup` session | state IDs, draft, selected modules/fields, back navigation, format dirtiness, random sample, review, consent and committed result |
| `internal/setup` effects | inspect/reinspect, entropy/build, preview, commit, cleanup, completion; typed results with operation IDs |
| `internal/setup` adapters | rich keys and basic lines translated into the same domain events; active component models, widths and transcript emission |
| `internal/config`, `internal/face` | strict random-face schema, deterministic build API and serialization |
| existing render/modules/playback | parsing, field registry, fetching, separator behavior and animation; reuse rather than duplicate |

The existing runtime exposes stdout TTY status but no stdin, stderr TTY,
startup cwd or entropy. Extend its inputs explicitly, grouping setup-specific
resources if needed. Separate TTY detection from width: the existing
`terminalColumns` returns zero for both unknown size and non-TTY, while init
must accept a TTY with unknown width and use 80 columns.

Use a typed session reducer for canonical events/guards and separate effect
functions. Generate no production code from the design JSON. Add a test that
accounts for every canonical state/transition, alongside independent behavior
tests; do not mistake transition coverage for correctness.

Command flow is inspect → destination/preset → face → modules → fields → order
→ colors/styles → shell → review preparation → review → explicit confirmation
→ commit → completion. Back and review-edit events follow machine.json rather
than a numeric step counter. Snapshot refresh preserves the candidate, updates
the target context and forces a new review and Yes.

Tag pending work with session revision and operation ID. Discard obsolete
preview/face results. Cancellation waits for worker termination and cleanup
before `cleanup.done`. A save worker's authoritative publication outcome must
be reconciled before classifying cancellation; never discard a successful
commit result as merely stale.

## Inline rendering, focus and inspection

| Surface | Component or application behavior | Size/focus |
| --- | --- | --- |
| Accepted answers, warnings, review TOML, preview | application-owned wrapped stderr transcript | immutable once emitted; no focus or clipping by terminal rows |
| Single choices and review actions | tuiweave `menu` with stable IDs | full choice-count height; one focused prompt |
| System modules | `menu` for navigation; app-owned checked set and Space handling | alphabetical display; selection order stored separately |
| Seed | tuiweave `textarea`, restricted to one logical line | natural wrapped content height; app intercepts Enter and rejects pasted controls/newlines |
| Numbered choices and order | basic line decoder; rich order may use wrapping editor | Enter submits; invalid content retained |
| Pending operations | tuiweave `spinner` in rich mode; one plain pending line in basic mode | intrinsic glyph; no percentages or focus |
| Confirm | inline No/Yes choices | No preselected; no dialog overlay |

Use one named theme, `tuiweave.CatppuccinMocha()`, with terminal-background inheritance
where appropriate. Chrome colors come from semantic roles. The configurable
smol palette remains product data interpreted by the existing renderer; it is
not the wizard theme. Nonempty NO_COLOR and TERM=dumb disable chrome color;
preview uses the candidate color mode with stderr capabilities.

Rich mode has one root `tea.View`, `AltScreen=false`, and no mouse capture,
outer frame, screen clearing or pinned footer. Emit completed prompt summaries
above the active view using Bubble Tea's inline printing command. Serialize
those emissions with prompt transitions so summaries appear once and in order.
Long review content must go through the transcript, not a viewport clipped to
terminal height. Clear the transient work line before completion output.

Compute the active block's natural height from wrapped text, choices, editor,
diagnostic and hints. Use `layout.NewRect(0, 0, width, naturalHeight)` and
`layout.Vertical(layout.Len(...), ...)`; call `SetSize` on every resize and
content-height change. Respect `SizeModeOf`: menus are bounded, the spinner is
intrinsic, and one-line controls do not fill an assigned height. Never allocate
the terminal's entire height. Maintain prompt-local rectangles for inspection.

Basic mode bypasses the repainting renderer and writes append-only ASCII
prompts with numbered input. Select it for TERM=dumb, absent cursor capability,
or width below 40; unknown width is 80. Wrap below 20 columns. Preserve answers
when resizing switches mode. Verify rich prompts taller than the available
display during the first slice; if Bubble Tea cannot preserve them, define a
tested append-only fallback rather than silently dropping rows.

Only the current prompt is focused. Intercept Enter/Esc/Ctrl+C before forwarding
keys; arrows only move the highlight, Space only toggles modules, and text
remains editable after rejection. Basic `:back`/`:cancel` apply only at choice
prompts, so literal seed values remain valid. EOF produces cancellation.
Reapply fresh component addresses if a focus manager is used after model copies.

Assemble a data-only `inspect` tree using visible canonical screen/component
IDs such as `modules/question`. Bind current component bounds; omit seed text,
TOML contents and environment values from generic inspection. Validate any
qualified semantic action against the current visible tree and enabled state
before forwarding its local suffix. No inspection server or public transport.

## Implementation slices

Execute sequentially. Each slice includes its failure paths and is verified
before the next. Shared files are intentionally owned by successive slices.

### 1. Prove the inline terminal adapter and invocation boundary

Goal: establish the stack exception, then build a cancellable destination-only
shell with independent init parsing and rich/basic adapters.

Reference tuiweave example: `examples/frame`; layout, menu, text editing and
focus recipes, adapted to inline rendering.

Components: root view, menu, wrapping editor, spinner, plain decoder.

Application-owned state: invocation, terminal capabilities, active prompt,
transcript emissions, input buffer and exit result.

Files allowed: `go.mod`, `go.sum`, `cmd/app/main.go`, `cmd/app/terminal_*.go`,
`cmd/app/*_test.go`, `internal/app/app.go`, its tests, and
`internal/setup/{invocation,terminal,view,input}*.go` plus testdata. Align SPEC's
dependency sentence only after the stack exception is accepted.

Behavior to preserve: normal CLI flags and output; init help needs no TTY or
config access. Invalid/duplicate/empty flags and extra positionals exit 2 before
inspection; stdin and stderr must be TTY even with stdout redirected.

Acceptance tests: help/usage matrix; unknown width; rich/basic same events;
accepted-answer scrollback; invalid input retained; resize including widths
19/39/40/80; long Unicode input; EOF, Ctrl+C and SIGTERM restore terminal.
Use a Linux PTY harness with a timeout to prove no alternate-screen or
screen-clear sequences and no repainting of accepted transcript. Verify EOF
actually ends the reader; do not rely on an assumed Bubble Tea EOF behavior.

Verification commands: `go test ./internal/setup ./internal/app ./cmd/app`,
`go vet ./...`, `go build ./...`; exact-toolchain dependency checks below.

Out of scope: saving, full navigation, changing ordinary signal behavior.

### 2. Add random-face schema and one-identity runtime resolution

Goal: support optional `face.random` without weakening deterministic behavior.

Reference tuiweave example: `examples/frame` command/result boundary; this
slice's schema/runtime work needs no UI component.

Components: none.

Application-owned state: injected entropy and one effective face identity per
invocation, shared by repeated tokens and animation frames.

Files allowed: `internal/face/*.go`, `internal/config/*.go`,
`internal/app/{app,expand}*.go`, `cmd/app/main.go`, `README.md`, `sample.env`.

Behavior to preserve: absent/false random uses existing selection; state is
validated even when disabled. True excludes present seed, including empty,
and nonempty preset. CLI preset/seed overrides clear random in memory.
Structural validation and config printing never consume entropy.

Acceptance tests: strict decode, source conflicts, omitted unset source keys,
round trips and default TOML; exactly 16 entropy bytes for rendered random
identity, none for hidden face; one identity for repeats/frames; entropy
failure follows ordinary unavailable-module handling, without stable fallback.
The current eager `face.Build` error exits the whole command, so explicitly
adapt random entropy failures to the renderer's unavailable behavior and
ensure animation has no invalid frame data when face generation is unavailable.

Verification commands: `go test ./internal/face ./internal/config ./internal/app`,
`make check`.

Out of scope: changing faceroll, new environment variables, animation controls.

### 3. Implement inspection and the editable session

Goal: build the reducer, safe read-only inspection, presets and all answer
editing through shell selection.

Reference tuiweave example: `examples/frame` menu result handling; text editing
and inspection recipes.

Components: choices, module navigation, wrapping seed/order editor, spinner.

Application-owned state: canonical context/draft/selection/field queue,
face mode/sample/expression, format dirty flag, cursor, errors and history.

Files allowed: `internal/setup/{session,update,inspect,presets,format,face,input}*.go`,
`presets/*.go`, setup tests/testdata; existing preset TOML stays authoritative.

Behavior to preserve: deep-copy maps/pointers before edits; preserve display,
module options, bold, inactive styles and untouched custom formats. Embed
shipped preset TOML in a Go file located inside `presets` so installed binaries
do not need the source checkout. Apply presence-aware key overlays rather than
parsing each preset as a complete default-filled replacement. Existing is
offered only for valid targets; malformed files warn; read failures block.

Acceptance tests: exact preset key effects and switching; 21 alphabetical
system toggles but preserved selection order; face-only selection; field queue
and back traversal; invalid permutation/field/style/seed; random keep/reroll
and stable every-run samples; no filesystem writes during any editing path.
Preserve repeated tokens and differing existing field selectors until a real
selection/field/order edit; derive the editable default from the first token
and leave the original format authoritative until then. Warn before the first
normalizing edit. Face enable/disable must operate on parsed tokens while
preserving unrelated literals and escaped braces, not by string replacement.

Verification commands: `go test ./internal/setup ./internal/config`,
`go vet ./...`.

Out of scope: display/module-option editors, save authorization or writes.

### 4. Prepare exact review bytes and shell instructions

Goal: connect editing to a validated, immutable review and default-No confirm.

Reference tuiweave example: `examples/frame` commands and semantic actions;
spinner recipe for preview work.

Components: transcript output, review menu, confirmation menu, spinner.

Application-owned state: reviewed bytes including final newline, target
fingerprint, static preview, instructions, draft revision and one-use consent.

Files allowed: `internal/setup/{review,shell,update,effects,view}*.go` and tests.

Behavior to preserve: reuse `config.TOML`, `render.Parse/Fetch/Assemble`, injected
module providers and the existing 20ms budget. Every-run review uses the
existing sample seed and never rerolls. Any edit/refresh invalidates review and
authorization. Commit must receive stored review bytes, never reserialize.

Acceptance tests: complete preserved TOML with exact trailing newline;
stderr-based preview auto-color; NO_COLOR versus explicit output policy;
unavailable modules; stale result rejection; new/replacement wording;
confirmation default No; Bash/Zsh/Fish quoting for apostrophes and metacharacters;
no shell command execution. Instructions name the correct startup file and
explain PATH and interactive banner behavior.

Verification commands: `go test ./internal/setup ./internal/render`,
`go vet ./...`.

Out of scope: writing files or changing configured output color semantics.

### 5. Implement atomic persistence, conflicts and awaited cleanup

Goal: publish exactly the reviewed bytes only after fresh Yes, with a
transaction result that distinguishes publication from later failures.

Reference tuiweave example: `examples/frame` typed result handling; filesystem
policy is entirely application-owned.

Components: transient save/cleanup status; conflict and error choices.

Application-owned state: fingerprint, consumed consent, temporary file and
created-parent ownership, pending cancellation, publication outcome.

Files allowed: `internal/setup/{store,store_linux,store_other,effects,update}*.go`,
fault-injection and real-filesystem tests; `go.mod`/`go.sum` only if directly
importing the already selected `golang.org/x/sys` is necessary for Linux calls.

Behavior to preserve: no temp files/parents/locks/backups before confirmation.
Walk components using directory handles and no-follow operations; reject
symlinks/special files and detect replaced ancestors. After authorization,
create missing parents 0700 and same-directory temp 0600; preserve existing
regular-file permission bits; write all bytes, sync, close and check errors.
Compare existence/device/inode/content hash/mode before writing and immediately
before publication. Use Linux no-replace rename for absent targets; unsupported
filesystems must fail safely rather than fall back to overwrite. Existing
targets use atomic replacement after the final check. Never claim this excludes
an uncooperative writer racing between that check and rename.

Acceptance tests: target/ancestor swaps, symlinks, FIFO/device, absent-target
creation race, mode/content/identity drift, short writes, mkdir/temp/write/
chmod/sync/close/rename failures, cancellation at every phase. Assert original
bytes survive prepublication failure; cleanup removes only owned temporary
paths and newly created empty parents, reporting exact leftovers on failure.
Refresh retains draft and requires review and new Yes. Serialize cancellation
with publication: once published, always deliver `save.committed`. If directory
sync or cleanup fails afterward, report saved status with the failure; never
offer another commit or claim rollback.

Verification commands: `go test -race ./internal/setup`, `make check`.

Out of scope: locks/backups, automatic merging, filesystem compare-and-swap
promises, non-Linux interactive setup support beyond a clear unsupported result.

### 6. Integrate completion and signal ownership

Goal: wire the complete flow to process resources and correct exit codes.

Reference tuiweave example: `examples/frame` root model; Charm inline output
and program lifecycle guidance.

Components: final transient-line cleanup and completion output.

Application-owned state: authoritative committed flag, signal cause, worker
completion, terminal restoration and final output errors.

Files allowed: `cmd/app/main.go`, `cmd/app/system*.go`, `cmd/app/*_test.go`,
`internal/app/app.go`, `internal/app/*_test.go`,
`internal/setup/{run,terminal,completion}*.go` and tests.

Behavior to preserve: normal render/playback signal behavior. The current
`cmd/app.run` unconditionally substitutes a signal exit code after `app.Run`;
init must instead retain its commit-aware result. Ensure one signal owner
coordinates Bubble Tea with the setup supervisor. Cancellation cannot kill the
UI before the save outcome and cleanup are known; precommit SIGTERM is 143,
EOF/Ctrl+C is 130, usage 2, operational/output error 1, help/success 0.

Acceptance tests: signals during entropy/preview/temp write and immediately
around publication; blocked reader cancellation; no late writes after cleanup;
restored terminal attributes/cursor on every exit; stdout empty before commit
and exactly absolute path plus newline on successful completion. Inject stdout
and stderr failures after publication: saved file remains and saved status is
reported on the remaining usable diagnostic path; exit 1, no retry/rollback.

Verification commands: `go test -race ./cmd/app ./internal/app ./internal/setup`,
`make check`.

Out of scope: changing non-init signal exits or adding machine-readable modes.

### 7. Finish scenario coverage, documentation and handoff

Goal: demonstrate the approved behavior across state, rendering, terminal and
filesystem boundaries and document the delivered command.

Reference tuiweave example: `snaptest` scenario tutorial and frame composition.

Components: all implemented prompt states and inspection trees.

Application-owned state: deterministic fixtures for time, modules, entropy,
filesystem snapshots, capabilities, signals and explicit command outcomes.

Files allowed: setup/app/command tests and testdata, `README.md`, `sample.env`,
and this plan's implementation evidence. Generated design artifacts change only
for an explicitly accepted semantic correction.

Behavior to preserve: canonical states/guards/defaults, inline scrollback and
all existing CLI regressions. `scenarios.json` currently contains visual state
fixtures; author real executable interaction scenarios rather than assuming
it supplies event sequences.

Acceptance tests: plain snapshots for all canonical states at 80 and 39 columns,
edge widths including 19, long paths/seeds and combining/wide characters;
paired `SnapCells` for representative semantic role states. Named
`RunScenario` checkpoints cover new-file success, preserved existing config,
every preset, each face mode, field navigation/normalization, malformed config,
retry, refresh/conflict, cancellation, commit races and output failure. Execute
returned commands and explicitly deliver their typed results: `RunScenario`
does not execute commands. Run PTY tests for rich/basic transcripts, terminal
restoration and signals. Review every intentional golden diff.

Verification commands: complete the handoff commands below; resolve findings
without bypassing project checks. Update docs for init, random semantics,
streams, preserved settings and the final check-to-rename race limitation.

Out of scope: recordings, browser previews, PR creation, CI/deployment changes.

## Verification and handoff

These are future implementation checks, not claims that implementation passed.
Use the actual Go 1.26.10 binary directory at the front of PATH and confirm
`GOTOOLCHAIN=local go version` reports `go1.26.10`. Then run:

```sh
GOTOOLCHAIN=local go mod tidy -diff
GOTOOLCHAIN=local go mod verify
GOTOOLCHAIN=local make check
GOTOOLCHAIN=local make audit
```

`make check` already runs vet, the pinned custom anti-slop linter, race tests,
the internal-package aggregate 80% coverage floor and binary build. Inspect
coverage for new setup failure paths as well as the enforced aggregate. Run
`make audit` because this feature introduces a dependency graph change.

Run the read-only kit audit at handoff:

```sh
GOTOOLCHAIN=local bash "$TUIWEAVE_AGENT_KIT/scripts/tuiweave-audit.sh" \
  --repo "$PWD" --strict --verify
```

`TUIWEAVE_AGENT_KIT` is a separately checked-out toolkit, not a new app
dependency. The inspected kit revision is
`bfe0f1dc15f442da86346a575dde062671736a39`; retain its companion version table.
Its scan traverses repository files, so distinguish application findings from
pre-existing vendored skill examples without deleting user files or weakening
the audit. Report unresolved findings honestly.

Handoff includes exact dependency revisions, executed checks and results,
reviewed golden diffs, PTY/fault-injection evidence, and any API gap with a
minimal reproduction and acceptance test. Do not modify tuiweave to close a
gap without explicit authorization.

## Implementation evidence

Delivered `internal/setup`, init dispatch and process resources, embedded preset
TOML, random-face validation/resolution, default-No confirmation, directory-handle
persistence, conflict refresh and commit-aware completion. Accepted answers go to
scrollback; oversized prompts and narrow/limited terminals use append-only
numbered input. Pending work is awaited, so its publication/cleanup outcome is
known before cancellation is reported.

Verified with Go 1.26.10 and `GOTOOLCHAIN=local`:

- `go mod tidy -diff` and `go mod verify`: passed.
- `make check`: passed, including custom lint, vet, race tests, build and 90.8%
  aggregate statement coverage; setup coverage is 86.2%.
- `make audit`: passed; no vulnerabilities found.
- PTY regression tests: basic and rich saves, rich cancellation, SIGTERM and
  narrow cancellation; terminal attributes restored, stdout isolated, no
  alternate screen or full-screen clearing.
- Reviewed 39 new plain, semantic-role and scenario goldens, including every
  editable prompt at standard/narrow widths. Fault tests cover write stages,
  destination/parent replacement, no-replace publication races, restrictive
  umasks, pre/postcommit cancellation and completion failures.
- `tuiweave-audit.sh --strict --verify`: all verification commands passed;
  summary is **0 blocking, 1 advisory**. Strict mode exits 1 because existing
  `.agents/skills/charmed-ecosystem/SKILL.md:186` and
  `.agents/skills/charmed-gum/SKILL.md:171` contain `@latest` examples. These
  unrelated skill files were preserved; the application dependency graph is
  pinned.

The small `internal/format` change moves its existing package-name lint
justification to the test package declaration where the current linter reports
it. No formatting behavior changed. Original user changes in the generated
specification bundle and skill directories remain in the working tree.
