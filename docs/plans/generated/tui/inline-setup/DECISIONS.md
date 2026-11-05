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

## Questions and answers

- Q-1 — Target task? User: Configure smol via smol init. Affects invocation, manifest, all views.
- Q-2 — Customization depth? User: Presets plus module order and colors; print shell setup instructions. Affects preset/modules/order/color/styles/shell contracts; no shell-file writes.
- Q-3 — Safety/automation? User: Interactive only; explicitly confirm replacement; no writes on cancel. Affects TTY checks, confirmation, config.commit, session.cleanup and exit states.
- Q-4 — Remaining defaults? Proposed preset lists, shell detection/confirmation and separate reorder/numbered fallback. User: Approved. Affects preset.submit, shell.submit, order.submit and prompt input semantics.
- Q-5 — Face scope? User: Include face setup. Proposed optional enablement, preset/seed, expression and static preview; incorporated into approved v3. Affects face views, fields and source model.
- Q-6 — Random timing? User: Both, up to the user. Distinct setup-random-once and runtime-random modes; affects face_mode, sample_seed, Face.random and entropy boundary.
- Q-7 — Final revised plan? User: Approved. Approves v3 including Face.random extension, source exclusivity, reroll and one character per invocation.

## Derived implementation defaults

These specify the approved behavior without expanding scope: 128-bit entropy represented as a seed; existing-file fingerprint checks plus atomic publication; new file mode 0600 and newly created parents 0700; safe shell quoting; no background processes, network operations, JSON mode or animation editing. Runtime lists come from existing face APIs. Every-run sampling is conditional on rendering a face. No new environment variable is proposed. Existing face CLI overrides clear random in effective memory only.

The no-writes-on-cancel promise applies before atomic publication. Signals arriving inside the publication critical section are deferred; after publication report saved status. Atomic existing-file replacement does not exclude an uncooperative writer racing the final snapshot check. This limitation is explicit in SPEC.md and config.commit.

## Superseded proposals

- v1 did not configure face; v3 includes it.
- The approved flow originally drew cancel/SIGTERM directly to terminal states. Validation requires a declared command outcome, so cleanup is modeled as synchronous `cancelling`/`terminating` → `cleanup.done` → terminal. User-visible semantics and exit codes are unchanged.
- The illustrative modifier-arrow reorder shortcut is replaced by the approved dedicated reorder prompt and numbered fallback.
- Single random interpretation is replaced by two user-selectable persistence modes.

## Open questions

None. This is a ready design specification, not implemented behavior. If future changes alter semantics, revise canonical files and obtain renewed scope approval per the invoked skill.
