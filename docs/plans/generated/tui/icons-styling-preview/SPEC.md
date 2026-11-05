# Icons, styles, and persistent setup preview

This package records the approved feature design implemented in the accompanying
Go changes. `contracts.json` is canonical for contracts; `screens.json` and
`machine.json` describe the presentation lifecycle. They abstract over individual
workflow questions and inherit destination inspection, review authorization,
conflict handling, and publication rules from the earlier
[inline setup specification](../inline-setup/SPEC.md).

The complete feature, configuration, and implementation sequence are in
[PLAN.md](PLAN.md). The implemented user-facing contract is summarized in
[CONTRACTS.md](CONTRACTS.md). These artifacts are documentation, not terminal
runtime evidence. Go unit, race, and PTY tests provide implementation validation.

Interactive setup occupies an inline stderr region: compact header, preview,
current question, feedback, and keyboard hints, in that order. No alternate
screen is used. Basic mode prints the header and preview before every question.
The preview uses at most two rows; overflow is visually clipped without changing
configuration. Narrow/short terminals transfer to numbered input after the
renderer stops, preserving the current input and multiselect state.

Up/down highlights choices; Space toggles multiselects; Enter validates and
accepts; Esc discards the tentative answer and navigates Back. Ctrl+C, Ctrl+D,
EOF, and signals cancel the workflow and await cleanup. A completed publication
still wins over concurrent cancellation. Invalid input remains editable and
retains the last valid preview. Color, icons, module order/fields, and face
choices preview without changing accepted configuration.

`smol init` requires TTY stdin/stderr, ignores XDG_CONFIG_DIRS for its destination,
and accepts `--config PATH`. Help works without TTY input. All prompts and
previews go to stderr. Stdout receives only the saved absolute path after
successful publication. Existing exit statuses remain 0, 1, 2, 130, and 143.
The presentation machine focuses on an already-validated interactive invocation;
startup/usage rejection with status 2 remains in the existing workflow contract.

The supplied `assets/logo-dark.png` and `assets/logo-light.png` are preserved
unchanged and used by the README. They supersede the originally proposed SVG
exports. FIGlet Standard lettering and sampled Phoenix palettes align the
terminal banner with the artwork. COLORFGBG hints at a light background; unknown
backgrounds use dark colors. The terminal is never queried for a font/background.
