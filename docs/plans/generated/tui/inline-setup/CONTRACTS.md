# Implementation contracts

`contracts.json` is canonical. All contracts are proposed. Type names are framework-neutral; runtime inputs, clocks, entropy, terminal I/O and filesystem operations must be injectable. See SPEC.md for the complete invocation, channel and commit semantics.

## UI contracts

Most views have one data-bound stderr output and at most one focused prompt; the transient `cleanup` view uses static stderr text. Completion has separate stdout/stderr nodes. Props resolve to declared state fields or the session, face_options, preset_options and field_options sources. Gesture events are semantic intent; the decoder maps rich or numbered input to the same events.

- `startup.content`: output at startup/content; emits none.
- `startup.pending`: spinner at startup/pending; emits none.
- `destination.content`: output at destination/content; emits none.
- `destination.question`: prompt at destination/question; emits destination.accept, wizard.cancel.
- `preset.content`: output at preset/content; emits none.
- `preset.question`: prompt at preset/question; emits preset.submit, back.preset, cursor.move, wizard.cancel.
- `face-mode.content`: output at face-mode/content; emits none.
- `face-mode.question`: prompt at face-mode/question; emits face.mode, back.face, cursor.move, wizard.cancel.
- `face-identity.content`: output at face-identity/content; emits none.
- `face-identity.question`: prompt at face-identity/question; emits face.identity, back.identity, cursor.move, wizard.cancel.
- `face-pending.content`: output at face-pending/content; emits none.
- `face-pending.pending`: spinner at face-pending/pending; emits none.
- `face-sample.content`: output at face-sample/content; emits none.
- `face-sample.question`: prompt at face-sample/question; emits face.accept, face.reroll, back.sample, wizard.cancel.
- `expression.content`: output at expression/content; emits none.
- `expression.question`: prompt at expression/question; emits face.expression, back.expression, cursor.move, wizard.cancel.
- `modules.content`: output at modules/content; emits none.
- `modules.question`: prompt at modules/question; emits modules.submit, modules.toggle, back.modules, cursor.move, wizard.cancel.
- `fields.content`: output at fields/content; emits none.
- `fields.question`: prompt at fields/question; emits fields.submit, back.fields, cursor.move, wizard.cancel.
- `order.content`: output at order/content; emits none.
- `order.question`: prompt at order/question; emits order.submit, back.order, wizard.cancel.
- `color.content`: output at color/content; emits none.
- `color.question`: prompt at color/question; emits color.submit, back.color, cursor.move, wizard.cancel.
- `styles.content`: output at styles/content; emits none.
- `styles.question`: prompt at styles/question; emits style.done, style.update, back.styles, cursor.move, wizard.cancel.
- `shell.content`: output at shell/content; emits none.
- `shell.question`: prompt at shell/question; emits shell.submit, back.shell, cursor.move, wizard.cancel.
- `preparing.content`: output at preparing/content; emits none.
- `preparing.pending`: spinner at preparing/pending; emits none.
- `review.content`: output at review/content; emits none.
- `review.question`: prompt at review/question; emits review.save, review.face, review.modules, review.colors, review.shell, back.review, cursor.move, wizard.cancel.
- `confirm.content`: output at confirm/content; emits none.
- `confirm.question`: prompt at confirm/question; emits confirm.submit, back.confirm, wizard.cancel.
- `saving.content`: output at saving/content; emits none.
- `saving.pending`: spinner at saving/pending; emits none.
- `finishing.content`: output at finishing/content; emits none.
- `finishing.pending`: spinner at finishing/pending; emits none.
- `load-error.content`: output at load-error/content; emits none.
- `load-error.question`: prompt at load-error/question; emits retry.inspect, wizard.abort, wizard.cancel.
- `face-error.content`: output at face-error/content; emits none.
- `face-error.question`: prompt at face-error/question; emits retry.face, back.sample, wizard.abort, wizard.cancel.
- `preview-error.content`: output at preview-error/content; emits none.
- `preview-error.question`: prompt at preview-error/question; emits retry.preview, review.modules, review.face, wizard.abort, wizard.cancel.
- `save-error.content`: output at save-error/content; emits none.
- `save-error.question`: prompt at save-error/question; emits retry.save, wizard.abort, wizard.cancel.
- `conflict.content`: output at conflict/content; emits none.
- `conflict.question`: prompt at conflict/question; emits conflict.refresh, wizard.abort, wizard.cancel.
- `refreshing.content`: output at refreshing/content; emits none.
- `refreshing.pending`: spinner at refreshing/pending; emits none.
- `result.content`: output at result/content; emits none.

## State and invariants

- `context: Context?` initially `null`. Inspected target and capability/environment snapshot; refreshed only by explicit commands.
- `destination: string` initially `""`. Resolved absolute path; fixed for the session; rerun --config to change.
- `preset: string` initially `"default"`. Chosen configuration basis.
- `draft: Config?` initially `null`. Editable complete config, including preserved face and style settings.
- `selected_modules: string[]` initially `[]`. Unique ordered enabled IDs; face appended on enabling if not already present.
- `selected_fields: map<string,string>` initially `{}`. Module to chosen field; absent means default module text.
- `field_queue: string[]` initially `[]`. Enabled modules with declared fields, in selection order, captured on `modules.submit`.
- `field_index: int` initially `0`. Index of the active prompt in `field_queue`.
- `face_enabled: bool` initially `false`. Whether format references face; disabling preserves inactive face options.
- `face_mode: string` initially `"seed"`. disabled, preset, seed, random-once, random-each-run.
- `identity: string` initially `"smol"`. Selected preset or typed seed.
- `sample_seed: string?` initially `null`. Ephemeral preview seed; persisted only for random-once.
- `face_sample: string` initially `""`. Static sample matching current identity and expression.
- `format_dirty: bool` initially `false`. Selection/field/order edits require normalized token-join format; otherwise preserve custom literals and field selectors.
- `shell_choice: string` initially `"none"`. bash, zsh, fish, none; detected SHELL is only a suggestion.
- `review_data: Review?` initially `null`. Exact bytes and preview; invalidated by edits.
- `authorized: bool` initially `false`. One-use consent bound to reviewed bytes and target fingerprint.
- `committed: bool` initially `false`. True only after atomic publication; cannot transition to cancellation terminal afterward.
- `error_text: string` initially `""`. Current prompt or command diagnostic; never written to stdout.
- `result_text: string` initially `""`. Saved path on success; no stdout content on precommit failure.
- `cursor: int` initially `0`. Current option highlight; UI navigation does not accept values.
- `no-preconfirmation-write`: NOT authorized AND NOT committed => no config/temporary/directory writes. Reads and previews cannot create filesystem state.
- `exclusive-face-source`: draft.face.random == true => draft.face.seed absent AND draft.face.preset absent; otherwise at most one of seed,preset. Three on-disk sources: preset, seed, random.
- `stable-random-once`: face_mode == random-once => saved face.seed == sample_seed AND saved face.random absent. The accepted character is repeatable.
- `runtime-random`: face_mode == random-each-run => saved face.random == true AND sample_seed is never serialized. Generate one ephemeral identity per future invocation.
- `one-character-per-run`: runtime random identity is resolved once and reused for all expression/frame renders in that invocation. No random change between frames or repeated face tokens.
- `module-uniqueness`: selected_modules has unique supported IDs; length >= 1 before leaving modules.ready; selected_fields keys are a subset of selected_modules. Face-only output is allowed.
- `field-uniqueness`: selected_fields has at most one entry per module, and every value is empty or a field declared by that module. One token per module, optionally with one field selector.
- `consent-binding`: commit input bytes == review_data.toml AND input snapshot == context fingerprint. Any edit or refresh clears authorization.
- `stream-separation`: stdout contains only saved absolute path + newline after commit; all interaction and diagnostics use stderr. No JSON mode or piped input.
- `cancel-before-commit`: terminal in {cancelled,terminated} => committed == false. Signals after the publication boundary cannot promise no writes.
- `preview-seed-stability`: review preparation does not change sample_seed. Every-run sample is stable until explicit reroll or identity change.

## Events

- `wizard.start` (system), payload `{}`: Process dispatches smol init after parsing its own invocation.
- `wizard.cancel` (ui), payload `{}`: Cancel/EOF before commit; restore terminal and exit 130.
- `wizard.terminate` (system), payload `{}`: SIGTERM before commit; clean up and exit 143.
- `input.invalid` (system), payload `{"message": "string"}`: Decoder rejects malformed input; show error and retain active prompt.
- `cursor.move` (ui), payload `{"delta": "int"}`: Move the current prompt selection without changing accepted answers.
- `inspect.ready` (command), payload `{"context": "Context"}`: Context and destination snapshot read; malformed TOML is reported in context, not hidden.
- `inspect.failed` (command), payload `{"message": "string"}`: Destination could not be inspected.
- `usage.failed` (command), payload `{"message": "string"}`: Invalid arguments, missing path, or stdin/stderr not TTY.
- `help.ready` (command), payload `{}`: Help printed without terminal checks or writes.
- `destination.accept` (ui), payload `{}`: Accept resolved destination and its inspection warning.
- `preset.submit` (ui), payload `{"preset": "string"}`: Choose existing, default, minimal, banner, prompt, or server.
- `face.mode` (ui), payload `{"mode": "string"}`: Choose disabled, preset, seed, random-once, or random-each-run.
- `face.identity` (ui), payload `{"value": "string"}`: Submit a known preset ID or printable nonempty seed.
- `face.built` (command), payload `{"sample": "string", "seed": "string?"}`: A static character sample is available.
- `face.failed` (command), payload `{"message": "string"}`: Character build or random entropy failed; never silently downgrade.
- `face.accept` (ui), payload `{}`: Accept this sample and continue to expression.
- `face.reroll` (ui), payload `{}`: Generate a fresh sample in either random mode.
- `face.expression` (ui), payload `{"expression": "string"}`: Choose a supported expression.
- `modules.toggle` (ui), payload `{"module": "string"}`: Toggle one of the 21 system modules. Face enablement is edited in Face setup.
- `modules.submit` (ui), payload `{}`: Continue with at least one enabled module; capture the field queue.
- `fields.submit` (ui), payload `{"module": "string", "field": "string"}`: Choose default text (empty field) or one declared field for the current module.
- `order.submit` (ui), payload `{"modules": "string[]"}`: Submit an exact permutation of enabled modules.
- `color.submit` (ui), payload `{"color": "string"}`: Choose auto, always, or never.
- `style.update` (ui), payload `{"module": "string", "fg": "string"}`: Set one enabled module foreground; retain existing bold.
- `style.done` (ui), payload `{}`: Keep current styles and continue.
- `shell.submit` (ui), payload `{"shell": "string"}`: Confirm detected Bash/Zsh/Fish, or choose no shell instructions.
- `preview.ready` (command), payload `{"toml": "string", "sample": "string", "instructions": "string"}`: Validated TOML, fingerprint, preview and instructions are ready.
- `preview.failed` (command), payload `{"message": "string"}`: Validation, serialization, or preview failed.
- `review.save` (ui), payload `{}`: Continue from review to explicit write confirmation.
- `review.face` (ui), payload `{}`: Return to face with draft answers retained.
- `review.modules` (ui), payload `{}`: Return to modules with draft answers retained.
- `review.colors` (ui), payload `{}`: Return to colors with draft answers retained.
- `review.shell` (ui), payload `{}`: Return to shell with draft answers retained.
- `confirm.submit` (ui), payload `{"yes": "bool"}`: Yes authorizes only the reviewed bytes and destination snapshot; No returns to review.
- `save.committed` (command), payload `{"path": "string"}`: Atomic replacement completed; cancellation can no longer promise no write.
- `save.failed` (command), payload `{"message": "string"}`: Write failed before commit; original target remains intact.
- `save.conflict` (command), payload `{"message": "string"}`: Target type, identity, bytes or permissions changed since review.
- `refresh.ready` (command), payload `{"context": "Context"}`: Updated destination fingerprint captured; draft retained.
- `refresh.failed` (command), payload `{"message": "string"}`: Conflict refresh could not inspect the target.
- `cleanup.done` (command), payload `{}`: Pending work stopped, stale outcomes discarded, uncommitted temporary files removed and terminal input restored.
- `finish.done` (command), payload `{}`: Saved path and instructions emitted successfully.
- `finish.failed` (command), payload `{"message": "string"}`: Output failed after a successful save; diagnostic must disclose saved file.
- `retry.inspect` (ui), payload `{}`: Retry read-only initial inspection.
- `retry.preview` (ui), payload `{}`: Retry validation and preview with draft preserved.
- `retry.face` (ui), payload `{}`: Retry character generation for selected mode.
- `retry.save` (ui), payload `{}`: Return to review; require fresh explicit write confirmation.
- `conflict.refresh` (ui), payload `{}`: Reinspect changed target without automatically overwriting it.
- `wizard.abort` (ui), payload `{}`: Stop after an operational failure with exit 1.
- `back.preset` (ui), payload `{}`: Go back from preset without persistence.
- `back.face` (ui), payload `{}`: Go back from face without persistence.
- `back.identity` (ui), payload `{}`: Go back from identity without persistence.
- `back.sample` (ui), payload `{}`: Go back from sample without persistence.
- `back.expression` (ui), payload `{}`: Go back from expression without persistence.
- `back.modules` (ui), payload `{}`: Go back from modules without persistence.
- `back.fields` (ui), payload `{}`: Go back one field prompt or to module selection without persistence.
- `back.order` (ui), payload `{}`: Go back from order without persistence.
- `back.color` (ui), payload `{}`: Go back from color without persistence.
- `back.styles` (ui), payload `{}`: Go back from styles without persistence.
- `back.shell` (ui), payload `{}`: Go back from shell without persistence.
- `back.review` (ui), payload `{}`: Go back from review without persistence.
- `back.confirm` (ui), payload `{}`: Go back from confirm without persistence.

## Commands and effect boundaries

A trigger names the primary intent; machine.transitions is the complete list of legal dispatch sites, including retries and rerolls. Cleanup is synchronous/awaited and emits `cleanup.done` once it has finished; cancelled workers otherwise have their stale outcomes discarded. Every command emits exactly one declared outcome after completion.

### config.inspect

Primary trigger `wizard.start`; inputs `{"argv": "Invocation", "environment": "Environment", "capabilities": "Terminal"}`. Outcomes: inspect.ready, inspect.failed, usage.failed, help.ready. Idempotent: True.

Read only: parse smol init [--config PATH] [--help], reject other options/positionals; help precedes TTY checks. Require stdin and stderr TTY. Resolve explicit path > XDG_CONFIG_HOME > HOME, reject absent destination; ignore XDG_CONFIG_DIRS for init writes. Lstat target and parent components; refuse symlinks/special files. Read bytes and mode, SHA-256 snapshot, parse with extended config schema. Malformed existing TOML becomes a visible replacement warning and disables Existing preset. Permission/read failures emit inspect.failed. No directories, lock files, temp files or config writes.

### config.reinspect

Primary trigger `retry.inspect`; same inputs, outcomes and read-only side effect as `config.inspect`. Idempotent: True.

### face.build

Primary trigger `face.identity`; inputs `{"mode": "string", "identity": "string", "expression": "string"}`. Outcomes: face.built, face.failed. Idempotent: True.

Build a static character using the current face preset/seed API and selected expression (idle initially). Pure computation; no persistence, animation or subprocesses.

### face.roll

Primary trigger `face.mode`; inputs `{"mode": "enum(random-once|random-each-run)", "expression": "string"}`. Outcomes: face.built, face.failed. Idempotent: False.

Read 16 bytes from injected OS entropy, encode as 32 lowercase hex characters, build using that seed and emit sample plus seed. A reroll reads fresh bytes. No seed is printed in every-run mode or persisted until save. Entropy failure is explicit. No fallback to a fixed face.

### face.reroll

Primary trigger `face.reroll`; same inputs, outcomes and entropy side effect as `face.roll`. Dispatched by the explicit Reroll action in either random mode. Idempotent: False.

### face.retry_build

Primary trigger `retry.face`; deterministic variant with the same build semantics as `face.build`, selected when the mode is preset or seed. Idempotent: True.

### face.retry_roll

Primary trigger `retry.face`; random variant with the same entropy semantics as `face.roll`, selected when the mode is random-once or random-each-run. Idempotent: False.

### review.prepare

Primary trigger `shell.submit`; inputs `{"draft": "Config", "sample_seed": "string?", "shell": "string", "context": "Context"}`. Outcomes: preview.ready, preview.failed. Idempotent: True.

Read-only validation and serialization of exact TOML, including preserved display/module options and `{module:field}` tokens; include random=true only in every-run mode, with seed/preset absent. Static system preview uses injected providers and the existing 20ms renderer budget; unavailable modules vanish with normal separator semantics. Every-run preview reuses session sample_seed: preparing review never rerolls. Return exact TOML bytes, sample and shell instructions; no config writes or subprocesses.

### review.retry

Primary trigger `retry.preview`; same read-only preparation semantics as `review.prepare`. Idempotent: True.

### review.refresh

Primary trigger `refresh.ready`; regenerate validated TOML, fingerprint, preview and instructions after a conflict refresh without rerolling the sample seed. Same read-only semantics as `review.prepare`. Idempotent: True.

### config.commit

Primary trigger `confirm.submit`; inputs `{"authorized": "bool", "review_data": "Review", "snapshot": "Context"}`. Outcomes: save.committed, save.failed, save.conflict. Idempotent: False.

Only after explicit Yes: recheck target snapshot (existence, device/inode, SHA-256 bytes, mode), reject unsafe targets and drift. Create missing parents with 0700, same-directory exclusive temp file 0600, write exact reviewed bytes, preserve existing regular-file permissions, sync and close; recheck snapshot immediately before atomic rename. For absent destination use atomic no-replace publication; never clobber a newly created file. Cancel before commit deletes temp and newly created empty parents. Existing-file replacement uses atomic rename; external writes in the last check-to-rename interval cannot be excluded without cooperating writers. Block cancellation across the short publication critical section; successful publication always emits save.committed, even if a signal arrives then. No automatic retries. No backups or shell-file edits.

### config.refresh

Primary trigger `conflict.refresh`; inputs `{"path": "string", "draft": "Config"}`. Outcomes: refresh.ready, refresh.failed. Idempotent: True.

Read-only target inspection and fingerprint refresh, same safe-target rules as initial inspection. Retain draft; malformed changed file is a warning and replacement still needs explicit review and confirmation.

### completion.print

Primary trigger `save.committed`; inputs `{"path": "string", "instructions": "string"}`. Outcomes: finish.done, finish.failed. Idempotent: False.

Write absolute saved path plus one newline to stdout. Write Saved confirmation and shell setup instructions to stderr, using the exact destination safely quoted. Output failure returns 1 and states that config WAS saved; never retry commit or remove a saved config.

### session.cleanup

Primary trigger `wizard.cancel`; inputs `{"pending_worker": "string?", "temporary_paths": "string[]"}`. Outcomes: cleanup.done. Idempotent: True.

Cancel workers, discard stale outcomes, remove uncommitted temp files and only newly created empty parents, restore input mode/cursor and finish the current stderr line. Never delete or alter a committed config. The `cancelling` state exists only until `cleanup.done`; `cancelled` then reports exit 130.

### session.terminate

Primary trigger `wizard.terminate`; same inputs and side effects as `session.cleanup`. Outcomes: cleanup.done. Idempotent: True.

SIGTERM variant. The `terminating` state exists only until `cleanup.done`; `terminated` then reports exit 143.

## Data models and sources

### Config

- `display: Display` (required): Display precision and unit controls; preserved unchanged by this wizard.
- `modules: ModulesConfig` (required): Per-module options; preserved unchanged by this wizard.
- `format: string` (required): Validated template; tokens may be `{module}` or `{module:field}`.
- `color: enum(auto|always|never)` (required): Output color mode.
- `style: map<string,Style>` (required): Per-module foreground and bold; the default palette covers every system module.
- `face: Face` (required): Extended face options; random flag is proposed new runtime behavior.

### Face

- `preset: string` (optional): Known named character; omitted when seed/random is selected.
- `seed: string` (optional): Deterministic identity; random-once stores generated seed here.
- `random: bool` (optional): NEW: true generates one identity per invocation; absent/false retains existing behavior.
- `state: string` (required): Known expression; default idle.

### Style

- `fg: string` (required): Terminal default empty string or known named color.
- `bold: bool` (required): Preserved from preset/existing config; no bold editor in this scope.

### Context

- `path: string` (required): Absolute destination.
- `exists: bool` (required): Target existence at inspection.
- `parseStatus: enum(absent|valid|invalid)` (required): Invalid TOML warned about explicitly.
- `fingerprint: string` (required): Existence/device/inode/content SHA-256/mode tuple.
- `config: Config` (optional): Existing validated config if parseable.
- `warning: string` (optional): Malformed TOML or target-change warning.

### Review

- `toml: string` (required): Exact bytes displayed and committed, including trailing newline.
- `sample: string` (required): Static system/face preview.
- `instructions: string` (required): Safely quoted shell commands; not executed.

### FaceOptions

- `presets: string[]` (required): From face.Presets(), sorted for presentation.
- `states: string[]` (required): From face.States(), idle first; remaining sorted.

### Session

- `context: Context` (optional): Resolved destination summary.
- `draft: Config` (optional): In-memory answers including untouched display and module options.
- `review: Review` (optional): Prepared review data.
- `error: string` (optional): Active diagnostic.
- `fieldQueue: string[]` (required): Modules with declared fields awaiting a choice.
- `fieldIndex: int` (required): Active field prompt.
- `selectedFields: map<string,string>` (required): Chosen field per module; absent means default text.

### Invocation

- `config: string` (optional): Only --config PATH or --config=PATH plus --help are accepted for init.
- `help: bool` (required): Help exits before TTY checks.

### Environment

- `XDG_CONFIG_HOME: string` (optional): Preferred config home.
- `HOME: string` (optional): Fallback home.
- `SHELL: string` (optional): Shell suggestion only.
- `NO_COLOR: string` (optional): Nonempty disables wizard chrome color.
- `TERM: string` (optional): dumb selects plain line-oriented numbered prompts.

### Terminal

- `stdinTTY: bool` (required): Required except help.
- `stderrTTY: bool` (required): Required except help.
- `stdoutTTY: bool` (required): May be redirected; does not govern prompting.
- `cols: int` (required): Wrapping width; unknown uses 80.

### Preset

- `id: enum(default|minimal|banner|prompt|server|existing)` (required): Stable preset identifier shown in the prompt.
- `label: string` (required): Short display label.
- `format: string` (optional): Format applied by this preset; absent for Existing.
- `color: enum(auto|always|never)` (optional): Color mode defined by default and minimal; otherwise preserved.
- `facePreset: string` (optional): Face preset applied by banner.
- `faceState: string` (optional): Expression applied by banner; default is idle.
- `diskMount: string` (optional): Disk mount applied by server.

### FieldOption

- `module: string` (required): Owning system module.
- `name: string` (required): Field selector accepted after a colon.
- `description: string` (required): User-facing field description from the module registry.

### Display

- `size: SizeOptions` (required): IEC/SI units and precision.
- `duration: DurationOptions` (required): Duration abbreviations and rounding.
- `percent: PercentOptions` (required): Percent digits and bar width.
- `temp: TempOptions` (required): Temperature unit and precision.

### SizeOptions

- `unit: enum(iec|si)` (required): Byte unit system.
- `digits: int` (required): -1 automatic or 0..6 places.

### DurationOptions

- `abbreviation: bool` (required): Short unit names.
- `spaceBeforeUnit: bool` (required): Space between value and unit.
- `compact: bool` (required): Omit zero units.
- `round: bool` (required): Round seconds into the next minute.

### PercentOptions

- `digits: int` (required): 0..6 decimal places.
- `width: int` (required): ASCII bar width, 1..100.

### TempOptions

- `unit: enum(c|f|k)` (required): Temperature unit.
- `digits: int` (required): 0..6 decimal places.

### ModulesConfig

- `disk: DiskOptions` (optional): Disk mount selection.
- `time: TimeOptions` (optional): Go time layout.
- `battery: BatteryOptions` (optional): Battery device selection.
- `localIP: LocalIPOptions` (optional): Interface and address family.
- `packages: PackagesOptions` (optional): Package database selection.

### DiskOptions

- `mount: string` (required): Mount point; default `/`.

### TimeOptions

- `format: string` (required): Go time layout; default `15:04`.

### BatteryOptions

- `device: string` (required): Power-supply name; empty selects the first battery.

### LocalIPOptions

- `interface: string` (required): Interface name; empty selects the first non-loopback interface.
- `family: enum(ipv4|ipv6)` (required): Address family.

### PackagesOptions

- `manager: enum(auto|dpkg|pacman|apk)` (required): Package database; auto probes in order.

### Sources

- `session` → Session (one), Injected in-memory wizard session: All screen data assembled from declared fields; no implicit I/O.
- `face_options` → FaceOptions (one), internal/face.Presets and internal/face.States: Read identifiers without mutating upstream registry; random modes are additional UI choices.
- `preset_options` → Preset (many), Built-in Default plus presets/minimal.toml, presets/banner.toml, presets/prompt.toml and presets/server.toml: Read-only preset definitions; Existing is offered separately when the target parses.
- `field_options` → FieldOption (many), modules.Get(name).Fields for enabled modules: Declared field selectors and descriptions; the registry is not duplicated.

## Validation

- `tty-required`, target `Terminal.stdinTTY`, when `wizard.start`: stdinTTY AND stderrTTY. Message: “smol init: interactive setup requires terminal stdin and stderr; use --print-default-config for manual setup.”
- `destination-required`, target `destination`, when `wizard.start`: resolved path is nonempty and has no control characters. Message: “smol init: cannot resolve config path; pass --config PATH.”
- `safe-target`, target `Context.path`, when `wizard.start`: target absent or regular file; no symlink in target/parent components. Message: “smol init: destination must be a regular file, not a symlink or special file.”
- `preset-choice`, target `preset`, when `preset.submit`: preset.valid. Message: “Choose Default, minimal, banner, prompt, server, or Existing.”
- `face-mode-choice`, target `face_mode`, when `face.mode`: mode in {disabled,preset,seed,random-once,random-each-run}. Message: “Choose a listed face mode.”
- `face-identity`, target `identity`, when `face.identity`: identity.valid. Message: “Choose a listed preset or enter a seed of 1–256 characters without controls.”
- `face-source-exclusive`, target `Face.random`, when `shell.submit`: random true excludes any seed field and any nonempty preset; seed and nonempty preset also mutually exclusive. Message: “face: random, preset and seed are mutually exclusive.”
- `face-preset`, target `Face.preset`, when `shell.submit`: absent or empty or member of face_options.presets. Message: “face: unknown preset; choose a listed preset.”
- `face-expression`, target `Face.state`, when `face.expression`: state in face_options.states. Message: “Choose a listed expression.”
- `selected-module`, target `selected_modules`, when `modules.toggle`: module.valid. Message: “Choose a system module; edit face in Face setup.”
- `nonempty-modules`, target `selected_modules`, when `modules.submit`: length >= 1. Message: “Select at least one module.”
- `field-choice`, target `selected_fields`, when `fields.submit`: event.module == field_queue[field_index] AND (event.field == "" OR event.field in field_options(event.module)). Message: “Choose default text or a listed field for this module.”
- `permutation`, target `selected_modules`, when `order.submit`: order.valid. Message: “List every enabled module number once.”
- `color-choice`, target `Config.color`, when `color.submit`: color in {auto,always,never}. Message: “Choose auto, always, or never.”
- `foreground-choice`, target `Style.fg`, when `style.update`: style.valid. Message: “Choose an enabled module and a listed foreground color.”
- `shell-choice`, target `shell_choice`, when `shell.submit`: shell in {bash,zsh,fish,none}. Message: “Choose Bash, Zsh, Fish, or No instructions.”
- `format-compatible`, target `Config.format`, when `shell.submit`: existing config.ValidateFormat with known modules and no control characters. Message: “smol init: invalid format; return to module selection.”
- `snapshot-match`, target `Context.fingerprint`, when `confirm.submit`: current fingerprint equals reviewed fingerprint and absent target still absent. Message: “Destination changed since review. Refresh and review before saving.”
- `consent-required`, target `authorized`, when `confirm.submit`: event.yes == true AND reviewed bytes and fingerprint still match. Message: “Save was not confirmed; no file was written.”
- `safe-target-commit`, target `Context.path`, when `confirm.submit`: target absent or regular file; no symlink in target/parent components. Message: “smol init: destination must be a regular file, not a symlink or special file.”
- `face-source-exclusive-commit`, target `Face.random`, when `confirm.submit`: random true excludes any seed field and any nonempty preset; seed and nonempty preset also mutually exclusive. Message: “face: random, preset and seed are mutually exclusive.”
- `face-preset-commit`, target `Face.preset`, when `confirm.submit`: absent or empty or member of face_options.presets. Message: “face: unknown preset; choose a listed preset.”
- `format-compatible-commit`, target `Config.format`, when `confirm.submit`: existing config.ValidateFormat with known modules and no control characters. Message: “smol init: invalid format; return to module selection.”
