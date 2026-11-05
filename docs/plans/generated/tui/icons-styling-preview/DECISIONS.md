# Decisions and authorization

- Q-1: User requested planning for icons, styling, and Nerd Fonts. Initial plan
  proposed opt-in icons, richer styles, wizard integration, and verification.
- Q-2: User supplied https://github.com/lrstanley/go-nf. Revised plan adopts it
  behind internal/icons, with a pinned dependency and no runtime font scans.
- Q-3: User requested a persistent wizard preview below the header. Revised
  plan includes one renderer, tentative answers, caching, and basic-mode fallback.
- Q-4: User requested FIGlet Standard, dark Phoenix F83600 through FCDF57, a
  darker light-theme gradient, logo assets, and README integration. Plan updated.
- Q-5: User requested "Write plan", then "Implement". These authorize the
  recorded feature scope and implementation; no additional approval is required.
- Q-6: During implementation the user supplied dark/light PNG artwork. Preserve
  those exact assets instead of generating the originally proposed SVGs. Sample
  matching terminal palettes from them; preserve the requested dark endpoints.
- Q-7: User reported warnings from bare make and then confirmed "Warning fixed".
  The assistant's bare make run passed without warnings; no warning suppression
  or check bypass was added.

Implementation choices: default icons remain never; auto is an explicit
TTY-based opt-in, not font detection. Icon overrides are literal text and can
remain TOML-only. Existing foreground-only editing is retained alongside the
new module style editor. Snapshot invalidation is conservative across all
cached modules when display/module inputs change. These routine choices were
made within the authorized implementation scope and are documented for review.

No unresolved product decisions block this implementation. Terminal/font
rendering differences remain documented platform limitations, not claimed
capabilities. The HTML explorer illustrates the contract, not a captured runtime.
