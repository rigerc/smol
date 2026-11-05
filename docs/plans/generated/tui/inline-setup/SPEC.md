# smol init: inline interactive setup

Status: approved proposed design (plan v4), schema 1.0.0. This package specifies a future feature; it does not implement it. `contracts.json` is canonical for API/validation/data, `screens.json` for structure, `machine.json` for transitions and updates. Markdown cannot override those files. Source context is isolated in SOURCE-CONTEXT.md; this is not a reconstruction of existing wizard behavior.

## Purpose and boundary

Help a local Linux user create or replace smol TOML without hand-editing. Configure presets, optional face, modules, per-module fields, order, color policy and foreground styles. Preserve existing display and module options. Print shell setup commands. Do not install smol, modify shell files, manage animation, add unrelated dependencies, fetch remote assets, or execute shell commands. No automatic first-run launch: normal `smol` stays a one-line command.

## Invocation and discovery

Proposed grammar: `smol init [--config PATH | --config=PATH] [--help]`. Parse init separately from normal invocation; do not forward the positional word into the current root flag parser. Unknown flags, duplicate --config, empty explicit paths, and extra positionals return 2 before inspection. --help returns 0 without TTY, config reads or writes. Init does not accept normal render options, --yes, --force or --json. Existing root flags remain outside this feature.

Destination precedence: explicit --config, then `$XDG_CONFIG_HOME/smol/config.toml`, then `$HOME/.config/smol/config.toml`. Resolve relative paths against injected startup working directory, without shell expansion; show absolute path. No HOME/XDG and no explicit path is an error. Refuse controls in paths and symlinks/special files in destination/parent components. Destination is fixed during the session; rerun with --config to change it.

`XDG_CONFIG_DIRS` and `/etc/xdg` are ignored by init: they are never offered as destinations, read as a basis, or merged. Normal `smol` read precedence is unchanged. Init always writes a user-owned path.

Require terminal stdin and stderr, even if stdout is redirected. Do not read /dev/tty to bypass redirected stdin. Reject pipes and redirected stderr with code 2 and a manual-setup hint. Only saved absolute path + one newline goes to stdout, after commit. Prompts, previews, progress, diagnostics, completion and shell instructions go to stderr. No JSON or other machine-readable mode is offered. stdout remains empty on all precommit failures/cancels.

## Inline interaction

No alternate buffer, screen clearing, outer border, pinned footer or terminal-height ownership. One active prompt at a time. Accepted answers become compact scrollback lines; only the current prompt may be repainted. Back appends the reopened question and retains answers. Enter accepts, Esc goes back (at destination it cancels), Ctrl+C cancels, EOF cancels; SIGTERM exits 143. Text editor preserves invalid input and shows a nearby explanation. Arrow movement highlights without submitting. Space toggles modules; Enter continues. Field prompts move a selection without submitting. Reordering has a separate prompt accepting a complete numbered permutation, not modifier-arrow shortcuts.

Rich terminal prompts may show a selection cursor. Basic mode (TERM=dumb, missing cursor capability, or width below 40 columns) uses append-only numbered choices; module input is a comma-separated set of numbers, field input is a number, order input a permutation. In basic mode type :back to go back and :cancel to cancel; these are controls only at choice prompts, not reserved seed values. Empty answer selects the displayed default. Destination defaults Continue, config preset defaults Existing when valid else Default, color defaults current value else auto, face defaults its existing enablement else disabled, expression defaults current value else idle, field defaults the current token's field else default text. Confirm always defaults No. Unknown terminal width uses 80; widths below 20 still use wrapping plain text rather than refusing. Rows never truncate transcripts. Long seeds/paths wrap, and diagnostics sanitize controls. Unicode separators are config text; basic wizard chrome is ASCII. Never rely on color alone.

NO_COLOR (nonempty) or TERM=dumb disables wizard chrome color even when the *configured output* mode is always. Auto preview color is based on stderr TTY and those environment inputs because preview is on stderr; explicit always/never follows normal renderer semantics. This is separate from future ordinary stdout color resolution. Restore input mode and cursor on every exit. Use only a transient status line during command work; no artificial percentages. Basic mode prints a single pending line. Clear/finish transient status before final output.

## Steps and defaults

1. Inspect destination: distinguish absent, parseable, malformed, unreadable and unsafe targets. Malformed TOML has an explicit warning, disables Existing and permits deliberate replacement from another preset. Unreadable targets block inspection with retry/exit.
2. Preset: Default, minimal, banner, prompt, server, and Existing when parseable. Default is preselected for a new file; Existing for a valid existing file. Each preset replaces only the keys it defines; later steps can edit them. For a new file the base is the built-in configuration, so display defaults, module option defaults, the full style palette and idle face state are present before the preset is applied.

   | Preset | Defines |
   | --- | --- |
   | `default` | format `{os} · {kernel} · {uptime} · {mem}`, color `auto` |
   | `minimal` | format `{os} · {kernel}`, color `never` |
   | `banner` | format `{face} {user}@{host} · {os} · {cpu} · {mem}`, face preset `kitty`, state `happy` |
   | `prompt` | format `{user}@{host} · {mem:percent}` |
   | `server` | format `{host} · {uptime} · {load} · {mem} · {disk}`, `[modules.disk] mount = "/"` |
   | `existing` | no replacements; preserves the parsed file |

   Presets never reset style or display tables. Choosing a different preset after edits applies its defined keys again; all other answers survive.
3. Face: Disabled, Preset, Seed, Random keep, Random every run. Enabling appends face if absent; existing position stays. Disabling removes every face token using parsed format semantics, retains inactive face options and does not reset unrelated literals. Face-only output is valid. Select preset from face.Presets(); seeded input defaults smol; new edited seeds are 1–256 Unicode scalars with no control characters. Existing valid seed values are preserved until edited. Presets and expressions come from current APIs, not a duplicated registry. Expression list presents idle first. Identity choices clear incompatible source fields.
4. Module selection: 21 system modules shown alphabetically as toggles; face is displayed with a link back to Face setup, not independently toggled. The supported set is battery, cpu, desktop, disk, host, init, kernel, load, localip, machine, mem, os, packages, processes, shell, swap, terminal, time, uptime, user, virt. Newly enabled system modules append. At least one module (including face) is required before continuing.
5. Field selection: after module selection, prompt once for each enabled module that declares fields, in selection order. Modules without fields are skipped; if none remain, go directly to order. Choices are default module text or one declared field from `modules.Get(name).Fields`. Existing field selectors are preselected. At most one field token per module. Empty answer keeps default text. Back moves to the previous field prompt, then to module selection.
6. Module order: exact permutation of enabled IDs. Labels append a chosen field, for example `mem (percent)`. If selection, fields and order stay unchanged, preserve custom format literals, repeated tokens and braces. Any selection, field or order edit normalizes the format to unique `{module}` or `{module:field}` tokens joined by ` · `; warn before the first such edit and show the exact result at review.
7. Colors: auto/always/never; optional foreground selection per enabled module. Allow empty string for terminal default, standard eight colors and bright variants. The built-in palette colors every system module, so newly enabled modules inherit a usable style. Preserve bold and valid inactive style entries. This wizard does not edit bold.
8. Shell: infer basename of SHELL for Bash/Zsh/Fish, but require confirmation; unknown defaults No instructions. Shell choice affects printed instructions only and is not a new persisted config field.
9. Review: serialized TOML (including preserved display, module, face and style values), destination, preview and replacement/creation warning. Display random mode semantics and selected field tokens. Offer Save, Edit face/modules/colors/shell or Cancel. Back returns to shell. Every edit invalidates any prior write authorization.
10. Confirm: `Create <path>? [y/N]` or `Replace <path>? [y/N]`; only an explicit Yes allows writing. No returns to review. A different snapshot always requires refresh/review and new confirmation.
11. Complete: stdout receives saved path; stderr receives completion and instructions. Exit 0 only when both save and output succeed.

## Display and module options

`[display.size]`, `[display.duration]`, `[display.percent]`, `[display.temp]`, `[modules.disk]`, `[modules.time]`, `[modules.battery]`, `[modules.localip]` and `[modules.packages]` are parsed with the existing schema and preserved through review and commit. This wizard does not offer editors for them. `config.TOML` semantics apply: default module option tables are omitted, and existing non-default values are serialized. A preset that defines a module option, such as server's disk mount, writes that value.

## Random face configuration extension

Add optional boolean `random` to [face]. `random = true` is mutually exclusive with a present seed (including empty seed) and a nonempty preset. Absent/false preserves existing selection behavior. Missing preset and seed with random absent/false still uses existing stable smol seed. State defaults idle and must be in face.States() even when face is disabled. Update strict decoder, validation, runtime identity resolution, --check-config, --print-default-config, README and sample.env descriptions as appropriate during implementation. No new environment variable is required.

Random keep: read 128 bits of injected OS entropy, encode 32 lowercase hex characters, preview and permit reroll, then persist the accepted seed; omit random and preset. Seed mode and random keep serialize identically; their different setup provenance need not be stored. Random every run: persist random=true and state, omit seed/preset. Setup keeps one ephemeral seed for a stable sample and rerolls only on explicit user request. Each future normal invocation resolves a fresh seed once, shares its character for repeated tokens/frames, and never writes it back to config. Ordinary execution samples entropy only if a face will actually render. Entropy failures are errors, not a stable fallback: wizard offers retry; normal rendering follows the existing unavailable-module behavior, while --check-config remains structural validation and needs no entropy. Existing CLI face-preset/face-seed overrides clear random in the effective in-memory config; they do not rewrite disk. Existing animation is not configured by init; an invocation using animation must keep its random character across all frames.

Every-run preview is labelled Sample — changes next run. Face expression previews are static; expression selection causes the next review to render the current sample seed with that expression. Reroll never changes the expression. No preview or entropy read creates a config file.

## Save, concurrency and cancellation

Before Yes: no config writes, temporary files, parent directory creation, lock files or backups. After Yes: use a same-directory temporary file, write reviewed bytes, sync/close, preserve an existing regular file's permissions (new file 0600; new parents 0700), then atomically publish. Use no-replace publication for absent targets. Recheck target identity/content/mode before writing and immediately before replacement. Any detected drift yields save.conflict, retains draft, and requires Refresh → Review → Confirm. Do not merge externally changed files automatically.

Atomic rename guarantees complete old/new file contents, not exclusion of uncooperative writers. An existing-file writer can race the final check-to-rename interval; document this limit rather than promising a compare-and-swap filesystem operation. Refuse changed symlink/unsafe paths and prefer directory-handle-relative operations to avoid parent traversal races. Publication is the commit boundary. Defer signals during its short critical section; once publication succeeds, always follow save.committed and report saved status, even if cancellation arrives then. Before that boundary, cancellation enters a brief `cancelling` state, removes temporary files and newly created empty directories, then reports exit 130; SIGTERM uses the equivalent `terminating` state and exit 143. Cleanup completion is the declared `cleanup.done` event, so the terminal result is shown only after cleanup returns. Cleanup errors are reported with exact leftover path; never claim removal succeeded if it failed.

Write failure before publication offers Review and retry or Exit. Retry requires fresh Yes; no automatic retries. Output failure after publication returns 1 and clearly states that the config was saved. Do not roll back or commit again. Startup/TTY errors are 2, operational failures 1, cancellation/EOF 130, precommit SIGTERM 143, help/success 0.

## Shell instructions

Print a one-time banner command to add manually to the chosen shell startup file, never a command that overwrites a shell file. Use `smol --config <quoted-path>`; this works for Bash/Zsh/Fish startup banners without prompt-string escape wrappers. Bash/Zsh instructions name ~/.bashrc or ~/.zshrc; Fish names ~/.config/fish/config.fish. Quote paths with shell-appropriate single-quote escaping; never interpolate unsanitized values. A literal executable name assumes smol is on PATH; tell the user this prerequisite. Explain that this prints a banner on interactive startup, not a PS1/prompt hook. No instructions means no snippet. All snippets are documentation, never executed by init.

## Acceptance criteria

- Success with no existing file, every preset, custom module selection and order, selected fields, colors and confirmed shell instructions; stdout contains exactly one absolute path line.
- Existing valid configs preserve unsupported-by-the-wizard but schema-supported values, including display and module option tables, custom format unchanged when unedited, bold and inactive styles. Malformed config is visibly warned about and replaced only after explicit confirmation.
- Presets apply exactly their defined keys; banner enables face kitty/happy, server sets the disk mount, minimal sets color never, and unrelated answers survive a preset switch.
- All four face identity choices round-trip; random keep persists accepted seed, reroll changes sample, every-run persists no sample seed and resolves once per runtime invocation. Existing deterministic configs stay deterministic.
- Field prompts appear only for enabled modules that declare fields; default text and field selectors round-trip; back navigation retains answers; field edits normalize to `{module:field}` tokens.
- Back/review edits retain answers; invalid options, empty module selection, invalid field selection, duplicate/missing order indices, unknown preset/state/color and conflicting face sources keep the user on a recoverable path.
- Cancellation/EOF/SIGTERM before commit, including pending entropy/preview/save work, restores terminal and leaves target unchanged. After publication, report saved status instead of claiming cancellation made no changes.
- Inject permission/full-disk/temp-write/close/sync/rename failures; preserve original bytes and require a new confirmation for retries. External drift and newly created targets do not silently overwrite without refresh/review.
- stdin pipes, non-TTY stderr and unknown arguments return 2; --help is usable without TTY. Redirected stdout receives only the final path. Broken output after commit reports the saved-file fact with exit 1.
- Verify 20/40/80-column transcripts, plain mode, NO_COLOR, TERM=dumb, unsupported arrow keys, safe quoting, and no alternate-screen or unrelated scrollback changes.

These are implementation acceptance checks, not tests executed by this design package.
