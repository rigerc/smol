# Presentation flows

1. Start setup and inspect the destination. Until a usable draft is loaded,
   the active region says Preview: preparing. A malformed existing file follows
   the existing explicit replacement flow.
2. Each workflow question enters editing with immutable accepted settings.
   Highlight/toggle/type computes a tentative candidate. Valid edits update the
   preview and request missing module values; invalid edits retain the last
   valid preview. Enter accepts and continues. Esc discards that answer.
3. Existing preset, face, module, field, and order steps are followed by icon
   mode/set, output color, style editing, and shell instructions. The new icons
   preset opts into automatic Nerd Font icons; existing presets do not.
4. Style editing selects a module, foreground/background or attributes, then
   returns to the property/module chooser. Custom colors validate names,
   indexed colors, and RGB values. Icon overrides remain editable in TOML and
   survive unrelated wizard edits.
5. Module reads run concurrently with navigation. Matching-generation results
   merge into the cache and reassemble the current candidate, never an old draft.
6. Resize reflows all active regions. If they cannot fit, stop the rich renderer
   and transfer current input to basic mode. Basic mode prints previews before
   questions and retains the existing :back/:cancel/none syntax.
7. Review shows exact TOML, destination, output preview, and edit routes.
   Save leads to explicit Create/Replace confirmation with No as default.
   Confirmed save uses the existing conflict, refresh/review, and atomic
   publication protocol.
8. Success exits 0 and prints only the saved path to stdout. Operational failure
   exits 1; prepublication cancellation exits 130; prepublication SIGTERM exits
   143. Startup usage rejection exits 2 outside the presentation machine.
   A committed save is reported as saved even if cancellation arrives concurrently.
