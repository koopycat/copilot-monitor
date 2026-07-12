package cli

import (
	"fmt"
	"io"
)

// When adding a new subcommand or flag, update the completionScript constant below.
const completionScript = `#compdef copilot-monitor

_copilot-monitor() {
  local -a commands
  commands=(
    'run:Start the local HTTP proxy listener'
    'serve:Start the read-only HTTP API and dashboard'
    'stats:Print captured usage grouped by model and endpoint'
    'cost:Print estimated equivalent provider list-price cost'
    'today:Print today'\''s captured usage'
    'sessions:Print captured sessions'
    'live:Print the current active session'
    'export:Export captured request metadata to CSV'
    'init:Create a starter routes.json config file'
    'validate:Validate a routes config file'
    'inspect:Show detected proxy anomalies'
    'version:Print the version'
    'help:Show help'
    'completion:Generate shell completion scripts'
  )

  _arguments -C \
    '(- *)'{-h,--help}'[Show help]' \
    '(- *)--version[Print version]' \
    '1: :_values "command" $commands' \
    '*::arg:->args'

  case $state in
    args)
      case $words[2] in
        run)
          _arguments \
            '--routes-config[path to routes configuration file]:file:_files' \
            '--addr[HTTP listen address]:address:' \
            '--db[SQLite database path]:file:_files' \
            '--project[filter by project]:project:' \
            '--usage-debug-log[optional JSONL path]:file:_files' \
            '--raw-log[optional JSONL path for raw request debugging]:file:_files' \
            '--no-live[disable live session tail]' \
            '--dashboard[serve dashboard API/UI on same port]' \
            '--log-format[log output format]:format:(human json)' \
            '--headroom-url[loopback Headroom compression endpoint]:url:' \
            '--headroom-timeout[Headroom compression request timeout]:duration:' \
            '--headroom-required[fail requests when Headroom is unavailable]' \
            '--headroom-compress-user-messages[allow Headroom to transform user messages]' \
            '--headroom-target-ratio[optional Headroom target ratio]:ratio:'
          ;;
        serve)
          _arguments \
            '--addr[HTTP listen address]:address:' \
            '--db[SQLite database path]:file:_files' \
            '--routes-config[optional JSON file with additional route definitions]:file:_files'
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
        init)
          _arguments \
            '--force[overwrite existing routes config]'
          ;;
        validate)
          _arguments \
            '--routes-config[path to routes configuration file]:file:_files'
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

_copilot-monitor "$@"
`

func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: copilot-monitor completion zsh")
		return 2
	}
	if args[0] != "zsh" {
		fmt.Fprintf(stderr, "unsupported shell %q; only \"zsh\" is supported\n", args[0])
		return 2
	}
	fmt.Fprint(stdout, completionScript)
	return 0
}
