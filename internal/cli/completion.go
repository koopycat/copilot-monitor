package cli

import (
	"fmt"
	"io"
)

// When adding a new subcommand or flag, update the completionScript constant below.
const completionScript = `#compdef copilot-monitor

_copilot-monitor() {
  local state state_descr line
  local -A opt_args

  local -a commands
  commands=(
    'run:Start the local HTTP proxy listener'
    'doctor:Check local setup'
    'serve:Start the read-only HTTP API and dashboard'
    'stats:Print captured usage grouped by model and endpoint'
    'cost:Print published token-rate estimate'
    'today:Print today'\''s captured usage'
    'sessions:Print captured sessions'
    'rebuild-sessions:Rebuild sessions from all requests'
    'live:Print the current active session'
    'export:Export captured request metadata to CSV'
    'inspect:Show detected proxy anomalies'
    'version:Print the version'
    'help:Show help'
    'completion:Generate shell completion scripts'
  )

  _arguments -C \
    '(- *)'{-h,--help}'[Show help]' \
    '(- *)--version[Print version]' \
    '1:command:->cmd' \
    '*::arg:->args'

  case $state in
    cmd)
      _describe 'command' commands
      ;;
    args)
      # With *:: (two colons), _arguments trims $words to normal
      # arguments, keeping the subcommand at $words[1].
      case $words[1] in
        run)
          _arguments \
            '--upstream[upstream host to proxy requests to]:host:' \
            '--headroom-proxy-addr[headroom compression proxy address]:address:' \
            '--addr[HTTP listen address]:address:' \
            '--db[SQLite database path]:file:_files' \
            '--project[filter by project]:project:' \
            '--usage-debug-log[optional JSONL path]:file:_files' \
            '--raw-log[optional JSONL path for raw request debugging]:file:_files' \
            '--no-live[disable live session tail]' \
			'--dashboard[start dashboard API/UI on separate port 7734]' \
            '--retention-days[days of requests and sessions to retain (0 disables)]:days:' \
            '--anomaly-retention-days[days of anomalies to retain (0 disables)]:days:' \
            '--dry-run[report retention deletions without executing them]' \
            '--log-format[log output format]:format:(human json)'
          ;;
        doctor)
          _arguments \
            '--db[SQLite database path to inspect]:file:_files' \
            '--proxy-url[local proxy base URL]:url:' \
            '--dashboard-url[local dashboard base URL]:url:' \
            '--skip-proxy[skip local proxy health check]' \
            '--skip-dashboard[skip local dashboard health check]' \
            '--upstream[optional upstream host to test]:host:' \
            '--timeout[timeout for local and upstream checks]:duration:' \
            '--json[emit machine-readable JSON]'
          ;;
        serve)
          _arguments \
            '--addr[HTTP listen address]:address:' \
            '--db[SQLite database path]:file:_files' \
            '--retention-days[days of requests and sessions to retain (0 disables)]:days:' \
            '--anomaly-retention-days[days of anomalies to retain (0 disables)]:days:' \
            '--dry-run[report retention deletions without executing them]'
          ;;
        stats)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--since[duration to look back]:duration:(24h 7d 30d all)' \
            '--project[filter by project]:project:' \
            '--endpoint[filter by endpoint]:endpoint:' \
            '--json[emit machine-readable JSON]'
          ;;
        cost)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--since[duration to look back]:duration:(24h 7d 30d all)' \
            '--project[filter by project]:project:' \
            '--endpoint[filter by endpoint]:endpoint:' \
            '--json[emit machine-readable JSON]'
          ;;
        today)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--project[filter by project]:project:' \
            '--endpoint[filter by endpoint]:endpoint:' \
            '--json[emit machine-readable JSON]'
          ;;
        sessions)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--since[duration to look back]:duration:(24h 7d 30d all)' \
            '--project[filter by project]:project:' \
            '--limit[maximum sessions to print]:limit:' \
            '--json[emit machine-readable JSON]'
          ;;
        rebuild-sessions)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--gap[inactivity gap used to split sessions]:duration:' \
            '--vacuum[compact the database after rebuilding sessions]'
          ;;
        live)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--json[emit machine-readable JSON]' \
            '--watch[refresh every 2s]'
          ;;
        export)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--since[duration to look back]:duration:(24h 7d 30d all)'
          ;;
        inspect)
          _arguments \
            '--db[SQLite database path]:file:_files' \
            '--since[duration to look back]:duration:(1h 24h 7d)' \
            '--category[filter by anomaly category]:category:(unrouted_path parse_error auth_missing unknown_content_type unknown_upstream unknown_ws_event)' \
            '--severity[filter by severity]:severity:(info warn error)' \
            '--json[emit machine-readable JSON]' \
            '--alert-on-any[exit 1 if any anomalies match]'
          ;;
        completion)
          _values 'shell' 'zsh'
          ;;
      esac
      ;;
  esac
}

# Register for users who source this file directly
compdef _copilot-monitor copilot-monitor

# Called by compinit when this file is autoloaded from fpath
if (( $# )); then
  _copilot-monitor "$@"
fi
`

func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: copilot-monitor completion zsh")
		return 2
	}
	if args[0] != "zsh" {
		fmt.Fprintf(stderr, "error: unsupported shell %q; only \"zsh\" is supported\n", args[0])
		return 2
	}
	fmt.Fprint(stdout, completionScript)
	return 0
}
