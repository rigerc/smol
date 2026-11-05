# Decisions and approval

## Plan review history

- v0: proposed smol init, destination → customization → preview/review → confirmation/save; inspected CLI/config source before interviewing. No artifacts generated.
- v1: user chose smol-specific init, presets plus module order/colors, printed shell instructions only, interactive-only setup, explicit replacement consent and no writes on cancel. Defined terminal and file-safety defaults.
- UI explanation: user asked how UI works; showed illustrative inline prompts, scrollback behavior, selection/toggle controls and provisional ordering fallback. This was not implementation evidence.
- Consolidated defaults: Minimal os/mem; Default os/kernel/uptime/mem; Detailed nine system modules. Detect SHELL then confirm Bash/Zsh/Fish. Separate reorder prompt, numbered fallback, Enter/Esc/Ctrl+C. User explicitly replied “Approved”.
- Before generation, user added “Include face setup”. v2 proposed optional face, preset or seed, expression, static preview and ordering; animation excluded. No files had been written.
- User added “Allow for random face”. Asked whether to keep accepted random face or generate a new one per invocation. User answered “Both, up to the user”.
- v3 proposed four identity modes (Preset, Seed, Random keep, Random every run), reroll and static sample, new face.random=true mutually exclusive with preset/seed, and one identity per future invocation. Presented full revised flow and requested approval.
- Final v3 sign-off: user explicitly replied **“Approved”**. Generation began only after that response.
- v4: re-inspected source after the module/display expansion. Observed drift: 21 system modules, `[display]`/`[modules]` tables, `{module:field}` syntax, shipped `presets/*.toml`, and XDG_CONFIG_DIRS read search. Proposed stale-anchor refresh, preset alignment, pass-through of new config tables, and field selection. No files written.
- v4 revision: user chose behavior-preserving refresh, repo presets, alphabetical module toggles, field selection, and ignoring XDG_CONFIG_DIRS. Revised plan added a field step and raised preset-application semantics.
- v4 consolidation: field step is per-module prompts with at most one field per module; presets apply full defined keys; Existing stays the default when valid; built-in Default remains the new-file default.
- Final v4 sign-off: user explicitly replied **"Approved and proceed"**. Canonical artifact update began only after that response.

## Questions and answers

- Q-1 — Target task? User: Configure smol via smol init. Affects invocation, manifest, all views.
- Q-2 — Customization depth? User: Presets plus module order and colors; print shell setup instructions. Affects preset/modules/order/color/styles/shell contracts; no shell-file writes.
- Q-3 — Safety/automation? User: Interactive only; explicitly confirm replacement; no writes on cancel. Affects TTY checks, confirmation, config.commit, session.cleanup and exit states.
- Q-4 — Remaining defaults? Proposed preset lists, shell detection/confirmation and separate reorder/numbered fallback. User: Approved. Affects preset.submit, shell.submit, order.submit and prompt input semantics.
- Q-5 — Face scope? User: Include face setup. Proposed optional enablement, preset/seed, expression and static preview; incorporated into approved v3. Affects face views, fields and source model.
- Q-6 — Random timing? User: Both, up to the user. Distinct setup-random-once and runtime-random modes; affects face_mode, sample_seed, Face.random and entropy boundary.
- Q-7 — Final revised plan? User: Approved. Approves v3 including Face.random extension, source exclusivity, reroll and one character per invocation.
- Q-8 — Update scope? User: Behavior-preserving refresh. Refresh provenance, module catalog and presets, and pass `[display]`/`[modules]` through unchanged; no display or module-option editors.
- Q-9 — Preset set? User: Adopt repo presets. Use minimal, banner, prompt and server from `presets/*.toml`; keep built-in Default as the new-file default.
- Q-10 — Module presentation? User: All toggles, alphabetical. Shows all 21 system modules; face stays separately configured.
- Q-11 — Format fields? User: Add field selection. Adds a field step and field-aware format normalization.
- Q-12 — XDG_CONFIG_DIRS? User: Ignore for destination. Init never offers, reads or merges system-wide configs.
- Q-13 — Field step shape? User: Per-module prompts. Only enabled modules that declare fields; modules without fields are skipped.
- Q-14 — Fields per module? User: At most one field. Order remains a permutation of modules, and each module contributes one token.
- Q-15 — Preset semantics? User: Apply full preset. Each preset replaces only the keys it defines; later steps may edit them.
- Q-16 — Preset versus existing? User: Existing first. Existing remains the default when the target parses; presets are deliberate replacements of their defined keys.
- Q-17 — New-file default? User: Keep built-in Default. It remains preselected when no config exists.

## Derived implementation defaults for v4

Presets: `default` defines format and color auto; `minimal` defines format and color never; `banner` defines format plus face preset kitty and state happy; `prompt` defines format; `server` defines format plus `[modules.disk] mount = "/"`; `existing` defines nothing. Style and display tables are never reset by a preset. A new file starts from the built-in configuration before the preset overlay.

Fields: the field queue is enabled modules in selection order that declare fields. Prompt choices are default text plus each declared field. Existing selectors are preselected. A field change marks the format dirty; order acceptance then joins `{module}` or `{module:field}` tokens with ` · `. Field prompts never appear for face or for modules with no fields.

Pass-through: display and module option values parsed from an existing file survive inspect, review and commit. Serialization follows existing `config.TOML` behavior, which omits default module option tables. Init ignores XDG_CONFIG_DIRS entirely.

## Superseded proposals

- v1 did not configure face; v3 includes it.
- The approved flow originally drew cancel/SIGTERM directly to terminal states. Validation requires a declared command outcome, so cleanup is modeled as synchronous `cancelling`/`terminating` → `cleanup.done` → terminal. User-visible semantics and exit codes are unchanged.
- The illustrative modifier-arrow reorder shortcut is replaced by the approved dedicated reorder prompt and numbered fallback.
- Single random interpretation is replaced by two user-selectable persistence modes.
- The v3 preset set Minimal/Default/Detailed is replaced by Default plus the repository's minimal, banner, prompt and server presets.
- The v3 nine-module catalog and the claim that presets replace only format are superseded by 21 modules and full-preset key application.
- The v3 behavior of dropping field selectors on normalization is superseded by the approved field step.

## Open questions

None. This is a ready design specification, not implemented behavior. If future changes alter semantics, revise canonical files and obtain renewed scope approval per the invoked skill.
