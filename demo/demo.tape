# VHS tape for the chcli demo GIF used in README.md.
#
# Regenerate:
#   make demo
# (which runs `vhs demo/demo.tape` from the project root)
#
# Prereqs:
#   - vhs installed (https://github.com/charmbracelet/vhs)
#   - A local ClickHouse on 127.0.0.1:9000 with the default user (no password)
#   - A built ./chcli binary at the repo root (make build)

Output demo/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1280
Set Height 720
Set Padding 20
Set Theme "TokyoNight"
Set TypingSpeed 80ms
Set PlaybackSpeed 0.9

Hide
Type "./chcli -h 127.0.0.1 -u default --database default"
Enter
Sleep 1.5s
Show

# 1) Build the demo's main query so both column and function completion
#    are visible. Scaffold the FROM clause first (that's the context the
#    completer needs), then move the cursor back so a column prefix
#    typed there filters columns of system.tables.
Type "SELECT  FROM system.tables GROUP BY engine ORDER BY tables DESC LIMIT 10;"
Sleep 700ms
Ctrl+A
Sleep 400ms
Right 7
Sleep 600ms

# Column completion: typing 'e' surfaces columns of system.tables
# starting with 'e' (engine, engine_full, …).
Type "e"
Sleep 2.5s
Type "ngine, "

# Function completion: typing 'cou' auto-shows the popup with count
# at the top (kind=AggFunction). Tab accepts: it inserts `count()` and
# parks the cursor between the parens. Right moves past the close
# paren before we type the alias.
Type "cou"
Sleep 2.5s
Tab
Sleep 1s
Right
Type " AS tables"
Sleep 800ms
Enter
Sleep 3s

# 2) Open the interactive table viewer on the result (Ctrl+T == F2).
Ctrl+T
Sleep 2.5s

# 3) Toggle to chart view. The numeric `tables` column gives a real
#    horizontal bar chart per engine.
Type "c"
Sleep 3s
Escape
Sleep 800ms
Escape
Sleep 1.5s

# 4) Toggle vertical display, recall the SELECT (two Ups — the first
#    brings back the \x we just typed; the second is the SELECT), and
#    re-run so the same rows render vertically.
Type "\x"
Enter
Sleep 1.2s
Up
Sleep 500ms
Up
Sleep 800ms
Enter
Sleep 3s
Type "\x"
Enter
Sleep 1s

# 5) Quit chcli. \monitor lives in a separate tape (demo/monitor.tape).
Type "\q"
Enter
Sleep 1.2s
