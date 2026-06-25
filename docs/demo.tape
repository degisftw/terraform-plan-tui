# docs/demo.tape — a reproducible demo of tfplantui, rendered with VHS.
#
#   Install the CLI:  make install        (puts tfplantui on your $PATH)
#   Install VHS:      brew install vhs    (pulls in ttyd + ffmpeg)
#   Generate:         vhs docs/demo.tape  (run from the repo root)
#
# Require fails fast if tfplantui isn't on $PATH. Output/example paths are
# relative to the repo root (VHS runs commands in your current shell).
# Swap the Output line to docs/demo.mp4 or docs/demo.webm for a real video.

Require tfplantui
Output docs/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

# Open straight into the bundled medium plan (create / replace / delete / no-op).
Type "tfplantui examples/medium/plan.json" Enter
Sleep 2s

# --- Navigate the dependency graph --------------------------------------
# Move across layers — the → key follows dependency edges, left → right.
Right
Sleep 1200ms
# Move within a layer (column) and back up.
Down@450ms 2
Sleep 1s
Up@450ms 2
Sleep 1s

# --- Expand a resource in place to reveal its before → after diff --------
Space
Sleep 2500ms
# Reveal the unchanged attributes too, then hide them again.
Type "u"
Sleep 2s
Type "u"
Sleep 1200ms
# Cycle through the instances of this count/for_each block.
Type "]"
Sleep 1200ms
Type "]"
Sleep 1200ms
# Collapse it.
Space
Sleep 800ms

# --- Keep moving across the graph, then filter to changes-only ----------
Right
Sleep 1200ms
Type "n"
Sleep 1500ms

# --- Flat list view, with jump-to-bottom / jump-to-top ------------------
Type "t"
Sleep 1200ms
Type "G"
Sleep 1200ms
Type "g"
Sleep 1200ms
Type "t"
Sleep 800ms

# --- Search by address and jump straight to a resource ------------------
Type "/"
Sleep 400ms
Type "master"
Enter
Sleep 1800ms

# --- Built-in help overlay ----------------------------------------------
Type "?"
Sleep 2500ms
Type "?"
Sleep 800ms

# --- Quit ---------------------------------------------------------------
Type "q"
Sleep 750ms
