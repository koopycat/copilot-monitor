#!/bin/sh
# Helper for the --no-live demo GIF. Sends a chat/completions request to the
# local proxy so the per-request log line appears in the proxy's terminal.
# Usage: sh demo/send-request.sh <model> <message>

MODEL="${1:-gpt-4.1}"
MSG="${2:-hello}"
curl -s -X POST "http://127.0.0.1:7733/copilot/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer demo" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"$MSG\"}],\"stream\":false}"
