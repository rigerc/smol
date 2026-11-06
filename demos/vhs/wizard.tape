# Run from demos/vhs/ so output paths are relative to the tape directory.
Output outputs/wizard.gif
Output outputs/wizard.mp4

Require smol
Set Shell bash
Set Width 1240
Set Height 680
Set FontSize 20
# DejaVu Sans Mono ships on the GitHub runner and covers the asciimoji face.
Set FontFamily "DejaVu Sans Mono"
Set Theme "Catppuccin Frappe"
Set TypingSpeed 55ms

# A short tour: choose the banner preset and follow its live preview through
# the face steps. The recording stops before saving.
Type "smol init --config /tmp/smol-demo.toml"
Enter
Wait+Screen /Set up your smol configuration/
Enter
Wait+Screen /Configuration preset/
Down
Down
Sleep 1s
Enter
Wait+Screen /Face mode/
Enter
Wait+Screen /Face preset/
Enter
Wait+Screen /Face sample/
Sleep 2s
