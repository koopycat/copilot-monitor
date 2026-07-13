# copilot-monitor demo
# Prerequisite: go run ./demo/seed/ (creates /tmp/demo.db)
# Prerequisite: just build-go (creates ./bin/copilot-monitor)
#
# Usage: vhs demo/copilot-monitor.tape

Output demo/copilot-monitor.gif

Set FontSize 16
Set Width 1050
Set Height 600
Set Theme "Dracula"
Set Padding 20
Set Framerate 12
Set PlaybackSpeed 1

Hide
Type "clear"
Enter
Sleep 200ms
Show

# Intro.
Type "# copilot-monitor — local LLM usage monitoring"
Enter
Sleep 1.5s
Type "./bin/copilot-monitor version"
Enter
Sleep 1.5s

# Today's usage.
Type "# Today's usage breakdown"
Enter
Sleep 500ms
Type "./bin/copilot-monitor today --db /tmp/demo.db"
Enter
Sleep 3s

# Full stats.
Type "# All captured data"
Enter
Sleep 500ms
Type "./bin/copilot-monitor stats --db /tmp/demo.db --since all"
Enter
Sleep 3.5s

# Estimated cost.
Type "# Estimated cost"
Enter
Sleep 500ms
Type "./bin/copilot-monitor cost --db /tmp/demo.db --since all"
Enter
Sleep 4s

# Live session.
Type "# Current session"
Enter
Sleep 500ms
Type "./bin/copilot-monitor live --db /tmp/demo.db"
Enter
Sleep 3.5s

# Tagline.
Type "# All data stays on your machine. No cloud, no telemetry."
Enter
Sleep 2.5s
