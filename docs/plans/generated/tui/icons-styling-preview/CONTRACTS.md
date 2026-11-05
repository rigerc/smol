# Feature contracts

`contracts.json` is canonical. See PLAN.md for the full configuration examples.

## Configuration and rendering

Icons default to mode never, set nerd-font, and a single separating space.
Modes auto/always/never and sets nerd-font/ascii are validated independently.
Auto follows the output TTY and TERM=dumb; always overrides those limitations.
NO_COLOR affects styling, not icons. CLI --icons and --icon-set override only
the corresponding settings. Unknown override modules, control characters, and
invalid UTF-8 are rejected. Empty overrides suppress both glyph and separator.

Styles support fg/bg, named/bright colors, index:0..255, #RRGGBB, and terminal
default empty strings. Attributes are bold, dim, italic, underline, and reverse.
Each module's explicit style replaces that module's default style. The existing
color policy gates all attributes. Icons and values share the same style and
shell escaping; unavailable tokens disappear before decoration. Distro icons
use the already-fetched OS ID. No font scans or fontconfig subprocesses run.

## State ownership and effects

The workflow exclusively owns accepted configuration and persistence. A question
contains an independent draft snapshot used by a pure tentative-answer builder.
Nested maps and module options are copied. The renderer owns controls, cached
module values, pending requests, and the last valid candidate/preview.

Selected module reads are bounded by the normal timeout and reused across
navigation/style edits. Changed module or display options invalidate the cache
as a whole, advancing its generation. Pending reads are deduplicated per module;
stale-generation completions are ignored. This conservative invalidation avoids
stale results while keeping ordinary style navigation free of reads.

Face previews are static, built locally, and reuse the session's random seed.
Reroll is explicit. Previewing never writes configuration, creates directories,
changes shell files, or grants publication authorization. Final review uses the
accepted draft and a fresh bounded snapshot before explicit confirmation.

## Components and lifecycle

Header and preview precede the active question. Controls retain their input and
focus through resize, and inspection coordinates include the persistent region.
Scrolling menus keep the active region bounded; terminals that cannot fit the
question, feedback and hints switch to basic mode. Basic mode is append-only.

The presentation machine's editing/working/basic states normalize the many
existing workflow substates. Self-transitions for draft.accept/back delegate
navigation to the existing workflow; these events do not imply automatic save.
A workflow.work message covers inspection, face generation, review preparation,
or confirmed save. The existing store remains the publication authority.

Text intended for scrollback crosses a serialized renderer bridge. Questions
and effect statuses use the same renderer; no nested interactive program runs
in rich setup. Cancel waits for the workflow to settle before restoring terminal
ownership. Success emits one absolute path line to stdout. Failure, cancellation,
and termination retain existing diagnostics/statuses.
