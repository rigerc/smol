# Flows and transitions

All behavior below is proposed. `machine.json` is authoritative. Updates run before commands. Commands finish by emitting exactly one declared outcome unless cancelled; cleanup is synchronous/awaited and emits `cleanup.done` after stale outcomes are discarded and temporary files are removed. Each command receives the post-update state and only one active generation may deliver outcomes.

## Primary scenarios

- New file: inspect → destination → preset → face → modules → fields (only when an enabled module declares fields) → order → colors → styles → shell → prepare → review → confirm Yes → commit → completion.
- Face disabled skips character/expressions. Preset/Seed builds a sample; either random mode rolls an ephemeral sample with optional reroll, then accepts expression.
- Presets: `default` is the built-in format; `minimal`, `banner`, `prompt` and `server` mirror the shipped `presets/*.toml` files. A preset applies only the keys it defines: format always; color for default/minimal; face preset and expression for banner; disk mount for server. Unrelated display, module, style and face values survive.
- Fields: one prompt per enabled module that declares fields. Accepting default text leaves `{module}`; choosing a field writes `{module:field}`. Back moves one prompt backward, then to module selection.
- Existing configuration: inspect/validate; Existing basis preserves data, or deliberately choose a new preset. Invalid TOML disables Existing, shows warning, and still requires explicit replacement confirmation.
- Save conflict: save.conflict → Refresh → updated target snapshot → regenerate review → confirm again. Candidate is retained, not merged.
- Precommit operational failure: retry, edit where offered, or Exit 1. Cancel/EOF enters a brief cleanup state, then exits 130. SIGTERM enters cleanup, then exits 143. Publication already completed always proceeds through completion with saved-file disclosure.

## States

| State | View | Terminal exit | Meaning |
| --- | --- | --- | --- |
| `entry` | `startup` | — | Process dispatch awaits wizard.start. |
| `inspecting` | `startup` | — | Inspection pending. |
| `destination.ready` | `destination` | — | Destination read and displayed. |
| `preset.ready` | `preset` | — | Choose basis. |
| `face.mode` | `face-mode` | — | Choose disabled or one of four character modes. |
| `face.identity` | `face-identity` | — | Choose preset or seed. |
| `face.building` | `face-pending` | — | Deterministic character build pending. |
| `face.rolling` | `face-pending` | — | Random sample build pending. |
| `face.sample` | `face-sample` | — | One face sample displayed. |
| `face.expression` | `expression` | — | Choose expression. |
| `modules.ready` | `modules` | — | Select modules. |
| `fields.ready` | `fields` | — | Choose default text or one field for the current module. |
| `order.ready` | `order` | — | Set order. |
| `color.ready` | `color` | — | Choose color policy. |
| `styles.ready` | `styles` | — | Keep or edit foreground styles. |
| `shell.ready` | `shell` | — | Choose instructions. |
| `preview.pending` | `preparing` | — | Candidate validation and preview pending. |
| `review.ready` | `review` | — | Exact candidate bytes and snapshot displayed. |
| `confirm.ready` | `confirm` | — | Explicit save authorization pending. |
| `save.pending` | `saving` | — | Write authorization consumed once. |
| `finish.pending` | `finishing` | — | Committed file; print result. |
| `load.error` | `load-error` | — | Read failure. |
| `face.error` | `face-error` | — | Character generation failure. |
| `preview.error` | `preview-error` | — | Review preparation failure. |
| `save.error` | `save-error` | — | Write failure before commit. |
| `save.conflict` | `conflict` | — | Destination drift or unsafe target. |
| `refresh.pending` | `refreshing` | — | Read updated target snapshot. |
| `done` | `result` | 0 | Saved config and printed completion. |
| `help` | `result` | 0 | Help printed; no interactive session. |
| `failed` | `result` | 1 | Operational failure before save. |
| `output.failed` | `result` | 1 | Config saved but output failed; diagnostic explicitly says so. |
| `usage.error` | `result` | 2 | Invalid CLI/TTY/path inputs; no writes. |
| `cancelling` | `cleanup` | — | Cancel requested; synchronous cleanup is in progress. |
| `cancelled` | `result` | 130 | Cancelled before commit; no saved config. |
| `terminating` | `cleanup` | — | SIGTERM received; synchronous cleanup is in progress. |
| `terminated` | `result` | 143 | SIGTERM before commit; cleaned up. |

## Transition table

| ID | From | Event | Guard | To | Commands | Updates/behavior |
| --- | --- | --- | --- | --- | --- | --- |
| t001 | `entry` | `wizard.start` | always | `inspecting` | config.inspect | Start read-only preflight. |
| t002 | `inspecting` | `inspect.ready` | always | `destination.ready` | — | Capture context and fingerprint. Updates: context = event.context; destination = event.context.path; error_text = "" |
| t003 | `inspecting` | `inspect.failed` | always | `load.error` | — | Complete preflight outcome. Updates: error_text = event.message if supplied else "" |
| t004 | `inspecting` | `usage.failed` | always | `usage.error` | — | Complete preflight outcome. Updates: error_text = event.message if supplied else "" |
| t005 | `inspecting` | `help.ready` | always | `help` | — | Complete preflight outcome. Updates: error_text = event.message if supplied else "" |
| t006 | `destination.ready` | `destination.accept` | always | `preset.ready` | — | Offer existing only if parseable; default to existing when valid, otherwise Default. |
| t007 | `preset.ready` | `preset.submit` | `preset.valid` | `face.mode` | — | Create draft from the selected basis; apply only keys the preset defines; clear write authorization. Updates: draft = apply_preset(basis(event.preset), event.preset); existing preserves all supported values; repository presets replace the keys they define; preset = event.preset; selected_modules = unique module references in draft.format; selected_fields = field selectors in draft.format for enabled modules; field_queue = enabled modules in selected_modules that declare fields; field_index = 0; face_enabled = draft.format contains {face}; face_mode = infer from draft.face, else disabled; format_dirty = false; authorized = false |
| t008 | `preset.ready` | `preset.submit` | `preset.invalid` | `preset.ready` | — | Show unavailable/unknown preset error. Updates: error_text = "Choose an available preset." |
| t009 | `face.mode` | `face.mode` | `mode.off` | `modules.ready` | — | Disable face; remove face tokens only if previously present. Updates: face_enabled = false; selected_modules = selected_modules without face; draft = remove face references from draft.format using parsed tokens; retain face options; authorized = false |
| t010 | `face.mode` | `face.mode` | `mode.identity` | `face.identity` | — | Configure deterministic identity. Updates: face_enabled = true; face_mode = event.mode; authorized = false |
| t011 | `face.mode` | `face.mode` | `mode.random` | `face.rolling` | face.roll | Create a new random sample. Updates: face_enabled = true; face_mode = event.mode; authorized = false |
| t012 | `face.mode` | `face.mode` | `mode.invalid` | `face.mode` | — | Reject unknown mode. Updates: error_text = "Choose a listed face mode." |
| t013 | `face.identity` | `face.identity` | `identity.valid` | `face.building` | face.build | Build chosen preset or seed. Updates: identity = event.value; authorized = false |
| t014 | `face.identity` | `face.identity` | `identity.invalid` | `face.identity` | — | Explain invalid preset or seed. Updates: error_text = "Choose a listed preset or enter a seed of 1–256 characters without controls." |
| t015 | `face.building` | `face.built` | always | `face.sample` | — | Retain sample seed for this setup session. Updates: face_sample = event.sample; sample_seed = event.seed if supplied else null; error_text = "" |
| t016 | `face.building` | `face.failed` | always | `face.error` | — | Preserve selected mode and show generation failure. Updates: error_text = event.message |
| t017 | `face.rolling` | `face.built` | always | `face.sample` | — | Retain sample seed for this setup session. Updates: face_sample = event.sample; sample_seed = event.seed if supplied else null; error_text = "" |
| t018 | `face.rolling` | `face.failed` | always | `face.error` | — | Preserve selected mode and show generation failure. Updates: error_text = event.message |
| t019 | `face.sample` | `face.reroll` | `random.active` | `face.rolling` | face.reroll | Roll a fresh sample. |
| t020 | `face.sample` | `face.reroll` | `random.inactive` | `face.sample` | — | Reroll unavailable for preset/seed; return to character selection to change them. Updates: error_text = "Choose a random mode to roll again." |
| t021 | `face.sample` | `face.accept` | always | `face.expression` | — | Accept sample; random-once persists seed, random-each-run does not. Updates: draft = reconcile face identity: preset OR seed(identity/sample_seed) OR random=true, never combinations; selected_modules = append face iff absent; preserve existing position otherwise; face_enabled = true |
| t022 | `face.expression` | `face.expression` | `expression.valid` | `modules.ready` | — | Accept expression. Updates: draft = draft with face.state = event.expression |
| t023 | `face.expression` | `face.expression` | `expression.invalid` | `face.expression` | — | Reject unknown expression. Updates: error_text = "Choose a listed expression." |
| t024 | `modules.ready` | `modules.toggle` | `module.valid` | `modules.ready` | — | Toggle module; preserve relative order, append newly enabled IDs, drop fields of disabled modules. Updates: selected_modules = toggle event.module in selected_modules; selected_fields = selected_fields restricted to the new selected_modules; field_queue = selected_modules filtered to modules with declared fields; format_dirty = true; authorized = false |
| t025 | `modules.ready` | `modules.toggle` | `module.invalid` | `modules.ready` | — | Reject unknown module; face enablement belongs to Face setup. Updates: error_text = "Choose a system module; edit face in Face setup." |
| t026 | `modules.ready` | `modules.submit` | `fields.pending` | `fields.ready` | — | Continue to field selection for enabled modules that declare fields. Updates: field_queue = selected_modules filtered to modules with declared fields; selected_fields = merge field selectors from draft.format with existing selected_fields, restricted to field_queue; field_index = 0 |
| t027 | `modules.ready` | `modules.submit` | `modules.empty` | `modules.ready` | — | Require at least one enabled module. Updates: error_text = "Select at least one module." |
| t028 | `order.ready` | `order.submit` | `order.valid` | `color.ready` | — | Accept permutation; preserve custom format if selection, fields and order are unchanged. Updates: format_dirty = format_dirty OR event.modules != selected_modules; selected_modules = event.modules; draft = if format_dirty or order changed, join {module} or {module:field} tokens with " · "; otherwise preserve format bytes |
| t029 | `order.ready` | `order.submit` | `order.invalid` | `order.ready` | — | Keep order input and show error. Updates: error_text = "List every enabled module number once." |
| t030 | `color.ready` | `color.submit` | `color.valid` | `styles.ready` | — | Accept color mode. Updates: draft = draft with color = event.color |
| t031 | `color.ready` | `color.submit` | `color.invalid` | `color.ready` | — | Reject unknown mode. Updates: error_text = "Choose auto, always, or never." |
| t032 | `styles.ready` | `style.update` | `style.valid` | `styles.ready` | — | Change foreground only; retain bold and other valid styles. Updates: draft = draft with style[event.module].fg = event.fg |
| t033 | `styles.ready` | `style.update` | `style.invalid` | `styles.ready` | — | Reject unlisted foreground or unavailable module. Updates: error_text = "Choose an enabled module and a listed foreground color." |
| t034 | `styles.ready` | `style.done` | always | `shell.ready` | — | Keep style values. |
| t035 | `shell.ready` | `shell.submit` | `shell.valid` | `preview.pending` | review.prepare | Prepare exact bytes and read-only preview. Updates: shell_choice = event.shell; authorized = false |
| t036 | `shell.ready` | `shell.submit` | `shell.invalid` | `shell.ready` | — | Reject unsupported shell. Updates: error_text = "Choose Bash, Zsh, Fish, or No instructions." |
| t037 | `preview.pending` | `preview.ready` | always | `review.ready` | — | Display candidate and preview. Updates: review_data = {toml:event.toml,sample:event.sample,instructions:event.instructions}; authorized = false; error_text = "" |
| t038 | `preview.pending` | `preview.failed` | always | `preview.error` | — | Keep draft and show error. Updates: error_text = event.message |
| t039 | `review.ready` | `review.save` | always | `confirm.ready` | — | Ask explicit Create or Replace question; default No. |
| t040 | `confirm.ready` | `confirm.submit` | `confirm.yes` | `save.pending` | config.commit | Authorize exactly reviewed bytes and fingerprint, once. Updates: authorized = true |
| t041 | `confirm.ready` | `confirm.submit` | `confirm.no` | `review.ready` | — | No returns to review without writes. Updates: authorized = false |
| t042 | `save.pending` | `save.committed` | always | `finish.pending` | completion.print | Commit complete; print completion exactly once. Updates: committed = true; authorized = false; result_text = event.path |
| t043 | `save.pending` | `save.failed` | always | `save.error` | — | Original intact; authorization invalidated. Updates: authorized = false; error_text = event.message |
| t044 | `save.pending` | `save.conflict` | always | `save.conflict` | — | No write; show drift and require refresh. Updates: authorized = false; error_text = event.message |
| t045 | `finish.pending` | `finish.done` | always | `done` | — | Success. |
| t046 | `finish.pending` | `finish.failed` | always | `output.failed` | — | Report output error and saved destination; do not roll back committed config. Updates: error_text = event.message |
| t047 | `load.error` | `retry.inspect` | always | `inspecting` | config.reinspect | Retry preflight. |
| t048 | `face.error` | `retry.face` | `random.active` | `face.rolling` | face.retry_roll | Retry randomness. |
| t049 | `face.error` | `retry.face` | `random.inactive` | `face.building` | face.retry_build | Retry deterministic build. |
| t050 | `preview.error` | `retry.preview` | always | `preview.pending` | review.retry | Retry review preparation. |
| t051 | `save.error` | `retry.save` | always | `review.ready` | — | Return to reviewed draft; repeat confirmation before trying to write. |
| t052 | `save.conflict` | `conflict.refresh` | always | `refresh.pending` | config.refresh | Read target again while retaining candidate. |
| t053 | `refresh.pending` | `refresh.ready` | always | `preview.pending` | review.refresh | Update snapshot, invalidate confirmation and regenerate review. Updates: context = event.context; destination = event.context.path; authorized = false |
| t054 | `refresh.pending` | `refresh.failed` | always | `save.conflict` | — | Report refresh error without losing draft. Updates: error_text = event.message |
| t055 | `load.error` | `wizard.abort` | always | `failed` | — | Exit after operational failure. |
| t056 | `face.error` | `wizard.abort` | always | `failed` | — | Exit after operational failure. |
| t057 | `preview.error` | `wizard.abort` | always | `failed` | — | Exit after operational failure. |
| t058 | `save.error` | `wizard.abort` | always | `failed` | — | Exit after operational failure. |
| t059 | `save.conflict` | `wizard.abort` | always | `failed` | — | Exit after operational failure. |
| t060 | `review.ready` | `review.face` | always | `face.mode` | — | Edit face; retain answers, invalidate review authorization. Updates: authorized = false |
| t061 | `review.ready` | `review.modules` | always | `modules.ready` | — | Edit modules; retain answers, invalidate review authorization. Updates: authorized = false |
| t062 | `review.ready` | `review.colors` | always | `color.ready` | — | Edit colors; retain answers, invalidate review authorization. Updates: authorized = false |
| t063 | `review.ready` | `review.shell` | always | `shell.ready` | — | Edit shell; retain answers, invalidate review authorization. Updates: authorized = false |
| t064 | `preview.error` | `review.modules` | always | `modules.ready` | — | Edit candidate after preparation failure. |
| t065 | `preview.error` | `review.face` | always | `face.mode` | — | Edit candidate after preparation failure. |
| t066 | `preset.ready` | `back.preset` | always | `destination.ready` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t067 | `face.mode` | `back.face` | always | `preset.ready` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t068 | `face.identity` | `back.identity` | always | `face.mode` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t069 | `face.sample` | `back.sample` | always | `face.mode` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t070 | `face.error` | `back.sample` | always | `face.mode` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t071 | `face.expression` | `back.expression` | always | `face.sample` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t072 | `modules.ready` | `back.modules` | always | `face.mode` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t073 | `order.ready` | `back.order` | `fields.pending` | `fields.ready` | — | Back to the last field prompt; retain draft. Updates: field_index = max(length(field_queue) - 1, 0); authorized = false |
| t074 | `color.ready` | `back.color` | always | `order.ready` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t075 | `styles.ready` | `back.styles` | always | `color.ready` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t076 | `shell.ready` | `back.shell` | always | `styles.ready` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t077 | `review.ready` | `back.review` | always | `shell.ready` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t078 | `confirm.ready` | `back.confirm` | always | `review.ready` | — | Back retains draft; selecting a different preset is the only explicit basis reset. Updates: authorized = false |
| t079 | `entry` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t080 | `entry` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t081 | `inspecting` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t082 | `inspecting` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t083 | `destination.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t084 | `destination.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t085 | `preset.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t086 | `preset.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t087 | `face.mode` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t088 | `face.mode` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t089 | `face.identity` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t090 | `face.identity` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t091 | `face.building` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t092 | `face.building` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t093 | `face.rolling` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t094 | `face.rolling` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t095 | `face.sample` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t096 | `face.sample` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t097 | `face.expression` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t098 | `face.expression` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t099 | `modules.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t100 | `modules.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t101 | `order.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t102 | `order.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t103 | `color.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t104 | `color.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t105 | `styles.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t106 | `styles.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t107 | `shell.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t108 | `shell.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t109 | `preview.pending` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t110 | `preview.pending` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t111 | `review.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t112 | `review.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t113 | `confirm.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t114 | `confirm.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t115 | `save.pending` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t116 | `save.pending` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t117 | `load.error` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t118 | `load.error` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t119 | `face.error` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t120 | `face.error` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t121 | `preview.error` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t122 | `preview.error` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t123 | `save.error` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t124 | `save.error` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t125 | `save.conflict` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t126 | `save.conflict` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t127 | `refresh.pending` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t128 | `refresh.pending` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t129 | `destination.ready` | `input.invalid` | always | `destination.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t130 | `destination.ready` | `cursor.move` | always | `destination.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t131 | `preset.ready` | `input.invalid` | always | `preset.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t132 | `preset.ready` | `cursor.move` | always | `preset.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t133 | `face.mode` | `input.invalid` | always | `face.mode` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t134 | `face.mode` | `cursor.move` | always | `face.mode` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t135 | `face.identity` | `input.invalid` | always | `face.identity` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t136 | `face.identity` | `cursor.move` | always | `face.identity` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t137 | `face.sample` | `input.invalid` | always | `face.sample` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t138 | `face.sample` | `cursor.move` | always | `face.sample` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t139 | `face.expression` | `input.invalid` | always | `face.expression` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t140 | `face.expression` | `cursor.move` | always | `face.expression` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t141 | `modules.ready` | `input.invalid` | always | `modules.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t142 | `modules.ready` | `cursor.move` | always | `modules.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t143 | `order.ready` | `input.invalid` | always | `order.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t144 | `order.ready` | `cursor.move` | always | `order.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t145 | `color.ready` | `input.invalid` | always | `color.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t146 | `color.ready` | `cursor.move` | always | `color.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t147 | `styles.ready` | `input.invalid` | always | `styles.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t148 | `styles.ready` | `cursor.move` | always | `styles.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t149 | `shell.ready` | `input.invalid` | always | `shell.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t150 | `shell.ready` | `cursor.move` | always | `shell.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t151 | `review.ready` | `input.invalid` | always | `review.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t152 | `review.ready` | `cursor.move` | always | `review.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t153 | `confirm.ready` | `input.invalid` | always | `confirm.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t154 | `confirm.ready` | `cursor.move` | always | `confirm.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t155 | `load.error` | `input.invalid` | always | `load.error` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t156 | `load.error` | `cursor.move` | always | `load.error` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t157 | `face.error` | `input.invalid` | always | `face.error` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t158 | `face.error` | `cursor.move` | always | `face.error` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t159 | `preview.error` | `input.invalid` | always | `preview.error` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t160 | `preview.error` | `cursor.move` | always | `preview.error` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t161 | `save.error` | `input.invalid` | always | `save.error` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t162 | `save.error` | `cursor.move` | always | `save.error` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t163 | `save.conflict` | `input.invalid` | always | `save.conflict` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t164 | `save.conflict` | `cursor.move` | always | `save.conflict` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t165 | `cancelling` | `cleanup.done` | always | `cancelled` | — | Cleanup completed after cancellation. |
| t166 | `terminating` | `cleanup.done` | always | `terminated` | — | Cleanup completed after SIGTERM. |
| t167 | `modules.ready` | `modules.submit` | `fields.none` | `order.ready` | — | No enabled module declares fields; continue directly to order. Updates: field_queue = []; selected_fields = {}; field_index = 0 |
| t168 | `fields.ready` | `fields.submit` | `fields.more` | `fields.ready` | — | Accept field choice; advance to the next field-capable module. Updates: selected_fields = if event.field == "" remove event.module else set selected_fields[event.module] = event.field; format_dirty = format_dirty OR event.field != current_field(draft.format, event.module); field_index = field_index + 1; authorized = false |
| t169 | `fields.ready` | `fields.submit` | `fields.complete` | `order.ready` | — | Accept last field choice; continue to order. Updates: selected_fields = if event.field == "" remove event.module else set selected_fields[event.module] = event.field; format_dirty = format_dirty OR event.field != current_field(draft.format, event.module); authorized = false |
| t170 | `fields.ready` | `fields.submit` | `field.invalid` | `fields.ready` | — | Reject an unknown field or out-of-sequence module; keep the prompt. Updates: error_text = "Choose default text or a listed field for this module." |
| t171 | `fields.ready` | `back.fields` | `fields.not-first` | `fields.ready` | — | Go back one field prompt; retain accepted choices. Updates: field_index = field_index - 1; authorized = false |
| t172 | `fields.ready` | `back.fields` | `fields.first` | `modules.ready` | — | Go back to module selection from the first field prompt. Updates: authorized = false |
| t173 | `fields.ready` | `wizard.cancel` | always | `cancelling` | session.cleanup | Cancel before commit; stop worker, discard late outcomes, remove uncommitted temp files, restore terminal. Updates: authorized = false |
| t174 | `fields.ready` | `wizard.terminate` | always | `terminating` | session.terminate | SIGTERM before commit; same cleanup, exit 143. Updates: authorized = false |
| t175 | `fields.ready` | `input.invalid` | always | `fields.ready` | — | Keep active prompt and entered value; display decoder feedback. Updates: error_text = event.message |
| t176 | `fields.ready` | `cursor.move` | always | `fields.ready` | — | Move current selection only. Updates: cursor = clamp or wrap to listed options using event.delta |
| t177 | `order.ready` | `back.order` | `fields.none` | `modules.ready` | — | No field prompts in this pass; go back to module selection. Updates: authorized = false |

## Guard predicates

- `preset.valid`: event.preset in {default, minimal, banner, prompt, server} OR (event.preset == existing AND context.parseStatus == valid)
- `preset.invalid`: NOT preset.valid
- `mode.off`: event.mode == disabled
- `mode.identity`: event.mode in {preset, seed}
- `mode.random`: event.mode in {random-once, random-each-run}
- `mode.invalid`: event.mode not in {disabled, preset, seed, random-once, random-each-run}
- `identity.valid`: (face_mode == preset AND event.value in face_options.presets) OR (face_mode == seed AND 1 <= UnicodeScalarCount(event.value) <= 256 AND no control characters)
- `identity.invalid`: NOT identity.valid
- `random.active`: face_mode in {random-once, random-each-run}
- `random.inactive`: NOT random.active
- `expression.valid`: event.expression in face_options.states
- `expression.invalid`: NOT expression.valid
- `module.valid`: event.module in {battery, cpu, desktop, disk, host, init, kernel, load, localip, machine, mem, os, packages, processes, shell, swap, terminal, time, uptime, user, virt}
- `module.invalid`: NOT module.valid
- `fields.pending`: length(selected_modules) >= 1 AND length(field_queue) > 0
- `fields.none`: length(selected_modules) >= 1 AND length(field_queue) == 0
- `field.invalid`: NOT (event.module == field_queue[field_index] AND (event.field == "" OR event.field in field_options(event.module)))
- `fields.more`: (event.module == field_queue[field_index] AND (event.field == "" OR event.field in field_options(event.module))) AND field_index + 1 < length(field_queue)
- `fields.complete`: (event.module == field_queue[field_index] AND (event.field == "" OR event.field in field_options(event.module))) AND field_index + 1 >= length(field_queue)
- `fields.not-first`: field_index > 0
- `fields.first`: field_index == 0
- `modules.empty`: length(selected_modules) == 0
- `order.valid`: event.modules contains every selected_modules ID exactly once and no others
- `order.invalid`: NOT order.valid
- `color.valid`: event.color in {auto, always, never}
- `color.invalid`: NOT color.valid
- `style.valid`: event.module in selected_modules AND event.fg in {empty string, black, red, green, yellow, blue, magenta, cyan, white, bright-black, bright-red, bright-green, bright-yellow, bright-blue, bright-magenta, bright-cyan, bright-white}
- `style.invalid`: NOT style.valid
- `shell.valid`: event.shell in {bash, zsh, fish, none}
- `shell.invalid`: NOT shell.valid
- `confirm.yes`: event.yes == true
- `confirm.no`: event.yes == false
