# VHS tape for the chcli \monitor demo GIF.
#
# Regenerate:
#   make demo-monitor
#
# Prereqs:
#   - vhs installed (https://github.com/charmbracelet/vhs)
#   - A local ClickHouse on 127.0.0.1:9000 with the default user
#   - A built ./chcli binary at the repo root (make build)

Output demo/monitor.gif

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

# Open the multi-tab monitor.
Type "\monitor"
Enter
Sleep 3s

# Tab 1 (Processes) is the default — let it render.
Sleep 2.5s

# Tab 2 (Merges).
Type "2"
Sleep 3s

# Skip Tab 3 (Charts) — its panels are driven by ClickHouse's
# system.dashboards table, which depends on the server actually writing
# system logs. The local demo CH may not have recent metric_log data.
# Jump straight to tab 4 instead.

# Tab 4 (Storage) — Databases → Tables → Partitions → Parts drilldown.
Type "4"
Sleep 4s

# Quit the monitor and chcli.
Type "q"
Sleep 1.2s
Type "\q"
Enter
Sleep 1.2s
