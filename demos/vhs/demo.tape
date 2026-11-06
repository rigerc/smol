# Run from demos/vhs/ so output paths are relative to the tape directory.
Output outputs/demo.gif
Output outputs/demo.mp4

Require smol
Set Shell bash
Set Width 1240
Set Height 320
Set FontSize 22
# Runner image and dev machines ship DejaVu Sans Mono; the asciimoji
# face below is picked (face-seed vhs) for full glyph coverage in it.
Set FontFamily "DejaVu Sans Mono"
Set Theme "Catppuccin Frappe"
Set TypingSpeed 60ms

Type "smol -face-seed vhs"
Enter
Wait+Screen /·/
# Hold the finished line long enough to read before the recording ends.
Sleep 3s

# The named PNG is extracted from the MP4 with 'ffmpeg -update 1'
# in the workflow: ffmpeg 8+ rejects single-image image2 output otherwise.
