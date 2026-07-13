# copilot-monitor no-live demo
# The proxy runs inside this terminal. Real HTTP requests are sent from
# outside (by demo/record-nolive.sh), so log lines appear live without any
# visible API-call commands.
#
# Usage: sh demo/record-nolive.sh

Output demo/copilot-monitor-nolive.gif

Set FontSize 15
Set Width 1050
Set Height 620
Set Theme "Dracula"
Set Padding 20
Set Framerate 12
Set PlaybackSpeed 1


# Start the proxy in background — banner + log output will appear here.
# The wrapper script will send real requests to port 7733.
Type "# Start proxy with --no-live"
Enter
Sleep 500ms
Type "./bin/copilot-monitor run --no-live --db /tmp/demo.db &"
Enter
Sleep 2.5s

Hide
Type '(KILO_GATEWAY_BASE_URL=http://127.0.0.1:7733/kilo pi -p "hello" &> /dev/null &)'
Enter
Type '(KILO_GATEWAY_BASE_URL=http://127.0.0.1:7733/kilo pi -p "hello" &> /dev/null &)'
Enter
Sleep 2s
Type "clear"
Enter
Show

Sleep 10s
