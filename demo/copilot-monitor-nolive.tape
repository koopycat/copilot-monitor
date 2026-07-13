# copilot-monitor no-live demo
# Shows per-request log output when running with --no-live. Each request that
# hits the proxy produces a log line with method, path, model, status, latency,
# and token counts.
#
# Prerequisite: go run ./demo/seed/ (creates /tmp/demo.db)
# Prerequisite: just build-go (creates ./bin/copilot-monitor)
#
# Usage: vhs demo/copilot-monitor-nolive.tape

Output demo/copilot-monitor-nolive.gif

Set FontSize 15
Set Width 1050
Set Height 620
Set Theme "Dracula"
Set Padding 20
Set Framerate 12
Set PlaybackSpeed 1

Hide
Type "clear"
Enter
Sleep 200ms
Show

# Start proxy in background with --no-live.
Type "# Start proxy with --no-live (per-request log)"
Enter
Sleep 500ms
Type "./bin/copilot-monitor run --no-live --addr 127.0.0.1:7733 --db /tmp/demo.db &"
Enter
Sleep 3s

# After the banner, log output appears when requests arrive.
Enter
Sleep 500ms

# Send a chat request via a helper script so the JSON doesn't break vhs parsing.
Type "# Agent sends a chat request"
Enter
Sleep 500ms
Type `sh demo/send-request.sh gpt-4.1 hello`
Enter
Sleep 3s

# Second request with a different model.
Type "# Another model — log shows model switch"
Enter
Sleep 500ms
Type `sh demo/send-request.sh claude-3.5-sonnet "refactor this"`
Enter
Sleep 3s

# Check captured data with live.
Type "# Check what was captured"
Enter
Sleep 500ms
Type "./bin/copilot-monitor live --db /tmp/demo.db"
Enter
Sleep 3s

# Shut down.
Type "# Shut down proxy"
Enter
Sleep 500ms
Type "kill %1 2>/dev/null; wait"
Enter
Sleep 2s
