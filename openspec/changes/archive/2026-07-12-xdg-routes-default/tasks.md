## 1. Implementation

- [x] 1.1 Add `defaultConfigPath()` helper that returns
      `$XDG_CONFIG_HOME/copilot-monitor/routes.json` or
      `~/.config/copilot-monitor/routes.json`
- [x] 1.2 Update `run.go` startup flow: try default config path when
      `--routes-config` is empty, fall back to built-in defaults if file
      missing/invalid
- [x] 1.3 Update startup banner to show config source (built-in, default file,
      explicit file)

## 2. Tests

- [x] 2.1 Test that default config file is loaded when it exists and no
      `--routes-config` flag
- [x] 2.2 Test that invalid default config file falls back to built-in defaults
- [x] 2.3 Test that `--routes-config` still overrides the default file
- [x] 2.4 Run full test suite with race detector

## 3. Spec update

- [x] 3.1 Update `openspec/specs/default-routes/spec.md` (archive step)
