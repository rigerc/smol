# Icons, styling, and persistent wizard preview

Status: implemented design. See CONTRACTS.md for final implementation choices
and DECISIONS.md for user authorization and supplied-artwork changes.

Add opt-in Nerd Font icons using `github.com/lrstanley/go-nf`, richer module
styles, and a persistent live preview directly below the setup wizard header.
Align the wizard banner and repository logo assets around FIGlet Standard and
the Phoenix gradient. Keep the ordinary default output unchanged.

## Scope and decisions

- Use go-nf's named glyph constants rather than maintaining raw codepoints.
- Provide Nerd Font and ASCII icon sets, per-module literal overrides, and
  independent icon visibility controls.
- Add background colors, dim, italic, underline, reverse, indexed colors, and
  RGB colors to the existing foreground/bold styles.
- Include these settings in the setup wizard, with immediate preview feedback.
- Keep setup inline, on stderr, without taking over the alternate screen.
- Keep the final TOML review and explicit save confirmation.
- Align the banner with logo assets using FIGlet Standard, a Phoenix gradient
  for dark backgrounds, and a darker red-to-amber gradient for light backgrounds.
- Add dark/light logo assets under `assets/` and display them in the README.
- Exclude Powerline segment layouts, borders, configurable labels, automatic
  font installation, independent icon styles, and animated wizard previews.

The user requested planning, supplied go-nf as the icon source, added the
persistent preview requirement, and authorized implementation after the written
plan. The defaults and edge-case rules below guided implementation; CONTRACTS.md
records final behavior and any adjustments.

## Current implementation and integration points

These observations are based on repository inspection; the rest of this document
describes proposed behavior.

- `internal/render/ansi.go`: `Style` currently supports `fg` and `bold`, with
  standard and bright named colors. Generated ANSI sequences are wrapped for
  Bash and Zsh prompts.
- `internal/render/render.go`: `Fetch` collects a module snapshot; `Assemble`
  selects fields, cleans values, removes unavailable references with their
  separators, escapes shell text, and applies styles.
- `internal/config/config.go`: strict TOML decoding, defaults, style validation,
  and the printed default configuration live here. Existing style entries
  replace a module's default style; preserve that behavior.
- `internal/app/app.go`: resolves color policy and passes render options. Face
  animation reuses fetched values and measures assembled unstyled output.
- `internal/setup/session.go`: owns the draft and sequential setup states.
- `internal/setup/prompt.go`: creates a separate inline Bubble Tea program for
  each question, with a numbered-input fallback for basic or small terminals.
- `internal/setup/review.go`: builds the output preview only at final review.
- `internal/setup/banner.go`: renders the current welcome banner with the
  `small` FIGlet font and a sunset/pink palette. No `assets/` directory exists
  in the inspected working tree; creating it is part of implementation.

Use the existing [inline setup specification](../inline-setup/SPEC.md) as context
for destination safety, cancellation, review, and publication behavior. This
feature changes the prompt presentation and settings, not the save protocol.

## 1. Icon configuration and policy

Proposed configuration:

```toml
[icons]
mode = "auto"       # auto | always | never; default is never
set = "nerd-font"   # nerd-font | ascii
separator = " "

[icons.overrides]
mem = "M"
host = ""           # suppress the icon for this module

[style.mem]
fg = "#a6e3a1"
bg = ""
bold = true
dim = false
italic = false
underline = false
reverse = false
```

The example opts into icons. With no icon configuration, mode is `never`, set is
`nerd-font`, separator is one space, and overrides are empty.

Add `--icons auto|always|never` and `--icon-set nerd-font|ascii`. Explicit flags
override their corresponding configuration fields in memory. Merely selecting
a set does not override visibility. Add these to help and config printing.

Resolve visibility independently from color:

| Mode | Behavior |
| --- | --- |
| `never` | No icons or icon separators |
| `auto` | Show on a TTY unless `TERM=dumb`; suppress for redirected output |
| `always` | Show even for redirected output or `TERM=dumb` |

Choosing `auto` or `always` is an explicit opt-in to the selected set. Auto mode
is an output policy, not a claim that a compatible font was detected. `NO_COLOR`
does not disable icons. Normal output checks stdout; wizard previews check stderr.
Prompt users who capture output for PS1 to select `always` when they want icons.

Validate mode, set, override module names, glyph strings, and separator with
actionable field-specific errors. Reject control characters, newlines, and invalid
UTF-8. Permit multi-rune printable overrides; do not assume one rune is one cell.
An empty override disables its module icon and its separator. Apply the existing
config error/fallback conventions and reject invalid explicit CLI options.

## 2. go-nf integration

Add a small `internal/icons` package containing icon options, validation, and
resolution. Keep third-party types behind that boundary. It must not depend on
`config` or perform terminal, filesystem, or environment I/O during resolution.

- Pin a verified release or commit-derived pseudo-version; do not track latest.
- Import only the glyph-family packages needed for the built-in module map.
- Provide a default icon for every system module and no default icon for face.
  An explicit face override is allowed.
- Use OS identifiers already present in fetched `os` fields for distro-specific
  icons. Unsupported identifiers fall back to the generic OS icon. Do not fetch
  extra modules just to select icons.
- Literal overrides take precedence over OS-specific and generic defaults.
- Keep ASCII fallback mappings local and independent of font availability.
- A normal invocation must not scan font directories or run `fc-list`.
- Setup may use injected environment/terminal hints to suggest a set. It must
  retain an explicit user choice and a visible glyph sample; installed fonts
  alone are insufficient evidence that the active terminal font supports them.

go-nf provides named constants, glyph-family packages, mappings, and installation
detection. Its installation status explicitly distinguishes installed fonts from
confirmed terminal use. Its documented API is pre-v1, so isolate it and record
the Nerd Fonts version represented by the chosen dependency.
Sources: [repository and usage](https://github.com/lrstanley/go-nf),
[detection implementation](https://github.com/lrstanley/go-nf/blob/master/detect.go).

## 3. Extended styles and renderer

Extend `render.Style` with `bg`, `dim`, `italic`, `underline`, and `reverse`.
Keep omitted attributes false. Support the same color syntax for foreground
and background:

- Existing standard and bright names.
- Empty string for terminal default.
- `index:0` through `index:255` for indexed colors.
- Exactly `#RRGGBB` for RGB colors, accepting either hex case.

Parse structured colors once when preparing render options. Emit deterministic
SGR sequences and reset after each styled module so attributes do not bleed
into separators or subsequent modules. Preserve existing byte sequences for
existing styles. Dim and bold may both be requested; terminal rendering varies.

Initially, indexed/RGB values emit their requested SGR form when color is
enabled. Document that terminals may approximate or ignore unsupported colors
and attributes. Automatic color-depth detection and quantization are follow-ups.
The existing color policy gates all styling, including non-color attributes.

Compose each nonempty token in this order:

1. Select and clean the module's value or requested field.
2. Skip an unavailable or empty value before attaching any icon.
3. Resolve the icon and optional separator, and join them to the value.
4. Escape the combined printable text for the requested shell.
5. Apply the module style to the whole combined value.

Every rendered token receives its icon, including repeated tokens and different
fields of the same module. Keep literal template text and module fetching
unchanged. Icons inherit the module style; independent icon styles are deferred.

Use identical options for normal output, setup previews, and animation. Extend
the existing plain-output measurement path to include icons and their separators.
Test the existing cell-width helper against selected Nerd Font glyphs and custom
Unicode. Recommend Nerd Font Mono and document terminal/font-dependent widths;
do not promise reliable width for every font variant.
Source: [Nerd Fonts font variant guidance](https://github.com/ryanoasis/nerd-fonts/wiki/FAQ-and-Troubleshooting#which-font).

## 4. Persistent wizard preview

### Layout and ownership

The active inline region has this ordered structure:

1. Compact wizard header identifying smol setup and the current step.
2. Output preview, immediately below the header.
3. Current question and its controls.
4. Validation feedback and navigation hints.

The header and preview must remain visible while moving between questions.
The existing large welcome banner may remain a one-time greeting; do not redraw
it on every keystroke. Keep all active-region rendering in one owner so prompt
program shutdown cannot erase the preview or append a duplicate preview block.

Refactor interactive setup toward one session-level Bubble Tea program. Reuse
the existing tuiweave controls, state transitions, and storage operations.
Run asynchronous inspection and reads as commands returning messages; keep
business state and I/O dependencies injected. Keep the numbered-input path as
the append-only fallback rather than forcing cursor control onto basic terminals.

### Draft and tentative state

Keep the accepted draft separate from the current tentative answer. Compute the
preview from a copy with the tentative edit applied. Highlighting a preset,
field, face, icon set, or style updates its sample before Enter; module toggles
and valid order edits do the same. Enter accepts; Back discards that tentative
answer and returns to the previous step with accepted settings intact.

If input is incomplete or invalid, keep the last valid preview and show the
validation error at the input. Do not silently mutate the accepted draft or
allow invalid input to advance. If a tentative edit would normalize a custom
format, show the existing normalization warning before acceptance.

At startup, show `Preview: preparing…` until destination inspection yields a
usable draft. Once loaded, show the current draft even on destination and shell
steps. An invalid existing configuration follows the current replacement flow;
its invalid contents are never rendered as a valid draft.

### Snapshot and responsiveness

- Fetch selected system values asynchronously under the existing shared timeout.
- Cache a session snapshot and reuse it across navigation and style edits.
- Fetch newly selected modules when needed, not on every highlight or keystroke.
- Invalidate only affected values when module or display options change; retain
  the original behavior for options the wizard does not edit.
- Tag requests with a revision and ignore stale completions after edits, Back,
  or cancellation. Prevent duplicate concurrent reads for the same request.
- Rebuild face samples locally; retain one random sample seed until reroll.
- Keep previews static even when the selected expression supports animation.
- Drop unavailable values with normal renderer semantics. If nothing renders,
  show `Preview: no available values` as wizard feedback only.
- Before save confirmation, regenerate the final preview from the accepted
  draft and completed bounded reads; do not save a tentative selection.

### Width, fallback, and output

Wrap the preview into at most two visible rows using ANSI-aware cell widths.
Truncate further overflow with an ellipsis in the wizard only. Recompute on
resize and preserve input focus and pending answers. Update inspection bounds
from the shared layout instead of retaining today's hardcoded prompt offsets.

If the active header, preview, question, feedback, and hints cannot fit, switch
to the existing basic input mode. In basic mode, print the header and current
preview before each question; do not attempt a pinned region or rewrite prior
scrollback. Preserve current input on a mid-question fallback.

Wizard chrome respects `NO_COLOR` and `TERM=dumb`. The output preview follows
the draft's explicit color and icon modes, including `always`, matching existing
preview color behavior. All wizard content remains on stderr. Stdout still
contains only the saved absolute path after successful publication.

Keep the preview visible but frozen during save work. Cancellation, EOF, signals,
save errors, external-file conflicts, cursor restoration, and publication
authorization retain the existing setup contracts. No preview operation writes
configuration, creates directories, or changes shell files.

## 5. Wizard settings and review

After module ordering, offer icon visibility, icon set, output color policy, and
optional per-module style editing. Show a glyph sample before enabling Nerd Font
icons. Provide named colors, terminal default, and a validated custom-color input
for indexed/RGB values. Expose the new style attributes as toggles.

Do not require users to step through every attribute of every enabled module:
choose a module to edit, preview its changes, then return to the module list or
continue. Per-module icon overrides can initially remain TOML-only; retain them
when editing other wizard settings. Preserve inactive styles and all schema-valid
settings the wizard does not expose.

The review menu gains Edit icons and Edit styles. Show the exact TOML, destination,
current preview, and existing create/replace warning. Keep explicit final
confirmation and conflict re-review. Returning from review retains all accepted
settings. No automatic save follows a preview or a style selection.

## 6. Banner and logo assets

Use the same FIGlet Standard lettering and palette definitions for the wizard
banner and repository logo. Replace the current `small` font and sunset/pink
stops rather than keeping a separate visual identity for setup.

- **Lettering:** render `smol` with FIGlet's `standard` font. Preserve its exact
  character layout and thin strokes in both terminal and exported logo assets.
- **Dark version:** use the supplied Phoenix stops, from `#F83600` through
  `#FCDF57`, to match the amber CRT look. Preserve the reference's intermediate
  stops and gradient direction when producing the assets; do not substitute
  the current pink palette. Only the endpoints are recorded in this request,
  so recover intermediate values from the reference if available and otherwise
  document the chosen interpolation during implementation.
- **Light version:** derive a darker red-to-amber version of the same gradient
  so the thin strokes remain readable on cream. Select and record its exact
  stops during asset work after checking the lettering at its intended size.
  Do not reuse the dark version's bright yellow endpoint unchanged on cream.
- **Shared source:** keep lettering and palette values in one authoritative
  definition or reproducible generation input. Terminal output and SVG exports
  must agree on font, glyph layout, gradient orientation, and theme palette.
- **Terminal theme:** use available terminal-background information through
  injected runtime inputs. If unknown, use the dark palette without blocking
  setup on a background query. Keep monochrome output for `NO_COLOR` and basic
  mode, with readable `smol` text when Standard lettering does not fit.
- **Layout:** keep the full Standard banner as the one-time greeting. Keep the
  persistent header compact and the output preview immediately below that
  header; the larger font must not crowd out the active question and hints.
- **Assets:** create `assets/logo-dark.svg` and `assets/logo-light.svg` with
  transparent backgrounds and outlined lettering so display does not depend
  on the reader having a particular font installed. Include accessible titles
  and descriptions and preserve readable padding around the artwork.
- **README:** display the matching asset using a theme-aware `<picture>` with
  a light/dark selection, a fallback image, meaningful `smol` alt text, and
  relative asset paths. Keep the introductory project description accessible
  below the logo.

Implementation update: the user subsequently supplied exact dark/light PNGs.
The implementation stores these unchanged as `assets/logo-dark.png` and
`assets/logo-light.png` and uses them in the README. They supersede generated SVG
exports. The terminal palettes are sampled from the supplied artwork; the
requested dark endpoints are retained. See DECISIONS.md.

## 7. Implementation sequence

1. **Dependency and icon boundary:** pin go-nf, inspect its selected APIs and
   dependencies, implement module/OS mapping and icon validation, run the audit.
2. **Configuration and rendering:** add icon schema and CLI overrides, richer
   color parsing and SGR generation, and icon-aware token assembly/width handling.
3. **Preview preparation:** extract reusable preview construction from final
   review; separate draft validation, module snapshot, face sample, and assembly.
4. **Wizard lifecycle:** move interactive region ownership to a session-level
   program, preserving existing navigation, fallback, and save behavior before
   adding tentative preview updates.
5. **Wizard controls:** add icon and style editors, persistent preview updates,
   async revisions, resizing, and review edit routes.
6. **Banner and assets:** switch to FIGlet Standard, define shared dark/light
   gradients, add the logo SVGs under `assets/`, and embed them in the README.
7. **Documentation and verification:** update examples and affected setup design
   contracts, then run the checks and terminal smoke cases below.

Each stage should leave existing configuration and normal invocation behavior
usable. Keep runtime feature work separate from unrelated UI restyling.

## 8. Verification and acceptance

Add meaningful tests for:

- Old configurations and defaults rendering unchanged; strict rejection of bad
  modes, sets, modules, colors, control characters, and invalid CLI overrides.
- TOML round trips, preservation of inactive styles/overrides, and precedence.
- All module mappings, OS fallback, empty overrides, repeated tokens, unavailable
  fields, ASCII output, and icon visibility independent of color.
- Exact SGR sequences, reset boundaries, indexed/RGB limits, and Bash/Zsh/Fish
  escaping of icons, separators, and values.
- Icon-inclusive animation width, resize fallback, and no per-frame fetching.
- Tentative selections, acceptance, Back, invalid input, normalization warnings,
  and stable random samples with explicit reroll.
- One cached read per unchanged module; bounded reads, late/stale completions,
  cancellation during preparation, and failure-as-absence behavior.
- Header/preview/question ordering throughout setup, review edit routes, and
  stdout remaining clean. Include layout snapshots at 80, 40, and 20 columns.
- Short terminals, live resize, ANSI-aware preview clipping, basic fallback,
  input/focus preservation, and cancellation/EOF cleanup through PTY tests.
- Existing confirmation, conflict, publication, and output-failure regressions
  after the wizard lifecycle refactor.
- Banner lettering matching FIGlet Standard; dark Phoenix endpoints and shared
  palette consistency between terminal output and assets; monochrome and
  narrow-terminal fallbacks without disturbing preview placement.

Visually review both logo variants on dark and cream backgrounds, especially
thin-stroke readability and padding. Verify the README's theme selection,
fallback, alt text, and relative asset links. Check that the SVGs render without
an installed FIGlet font and agree with the terminal banner's lettering.

Update README, `config.DefaultTOML`, presets, CLI help, and `sample.env` where
behavior changes. Document font setup, icon policy for captured shell prompts,
color syntax, terminal limitations, and basic-mode preview behavior. Add an
explicit icon-enabled preset without enabling icons in existing presets.

Run `make fix`, `make check`, and `make audit` during implementation. Manually
smoke-test a compatible font, an ordinary font with ASCII icons, redirected
output, shell prompt embedding, `NO_COLOR`, `TERM=dumb`, and terminal resizing.
The wizard lifecycle refactor is the largest regression risk; complete its
navigation and save-path checks before polishing the new settings.
