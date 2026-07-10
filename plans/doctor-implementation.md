# Doctor Command — Detailed Implementation Plan

**Phase 1, Chunk 1.1** | Depends on: 0.5, 0.6, 0.8 | Effort: M

---

## 1. Command Structure

### Entry Point

Registered in `internal/cli/root.go` alongside existing commands.

```
llm-proxy doctor --routes-config path [--db path] [--check-connectivity] [--no-color]
```

### Flags

| Flag                   | Type   | Required | Default               | Description                                                                 |
| ---------------------- | ------ | -------- | --------------------- | --------------------------------------------------------------------------- |
| `--routes-config`      | string | **Yes**  | —                     | Path to the JSON routes config file                                         |
| `--db`                 | string | No       | `store.DefaultPath()` | SQLite database path                                                        |
| `--check-connectivity` | bool   | No       | `false`               | Opt-in TCP probe of upstream hosts (3s timeout each)                        |
| `--no-color`           | bool   | No       | `false`               | Disable colored output (also respects `NO_COLOR` env, `TERM=dumb`, non-TTY) |

### Exit Codes

| Code | Meaning                                                          |
| ---- | ---------------------------------------------------------------- |
| `0`  | All checks passed                                                |
| `1`  | One or more checks failed                                        |
| `2`  | Usage error (missing required `--routes-config`, bad flag, etc.) |

---

## 2. Check Definitions

### Check 1: Version — `checkVersion() checkResult`

- **Always passes.**
- Prints `"Version"` as Name, `"ok"` as Status.
- Detail: `fmt.Sprintf("llm-proxy %s", version)` where `version` is the existing
  `var version = "0.1.0-dev"` in `root.go`.

```
→ checkResult{Name: "Version", Status: "ok", Detail: "llm-proxy 0.1.0-dev"}
```

### Check 2: Routes config valid — `checkRoutesConfig(path string) checkResult`

- Calls `proxy.LoadConfig(path)` (which reads, strips comments, parses JSON, and
  validates).
- On success: `Status = "ok"`,
  `Detail = fmt.Sprintf("valid (%d routes) [%s]", len(cfg.Routes), path)`.
- On failure: `Status = "fail"`, `Detail = err.Error()`. The error from
  `LoadConfig` already uses the labeled error format from chunk 0.5, e.g.
  `route "chat" (/v1/chat/completions): upstream_host is required`.

```
→ success: checkResult{Name: "Routes config", Status: "ok",  Detail: "valid (3 routes) [/path/to/routes.json]"}
→ failure: checkResult{Name: "Routes config", Status: "fail", Detail: "read routes config \"/bad\": read ..."}
```

The caller must also retain the parsed `*proxy.ProxyConfig` for downstream
checks — refactor `checkRoutesConfig` to return
`(*proxy.ProxyConfig, checkResult)` or store the config in a struct field.

### Check 3: Database writable — `checkDatabase(dbPath string) checkResult`

- Calls `store.Open(dbPath)` and immediately `defer s.Close()`.
- `store.Open` already creates the parent directory (`os.MkdirAll`) and runs
  `PRAGMA journal_mode=WAL` + schema migration — this is sufficient proof of
  writability.
- On success: `Status = "ok"`, `Detail = store.FormatPath(dbPath)`.
- On failure: `Status = "fail"`, `Detail = err.Error()`.

```
→ success: checkResult{Name: "Database", Status: "ok",  Detail: "/Users/me/.local/share/llm-proxy/store.db"}
→ failure: checkResult{Name: "Database", Status: "fail", Detail: "read-only file system"}
```

### Check 4: Disk space — `checkDiskSpace(dbPath string) checkResult`

1. Compute the directory containing DB: `dir := filepath.Dir(dbPath)`.
2. Check directory exists: `os.Stat(dir)` — if not exists, return fail
   `"directory does not exist: <dir>"`.
3. Call `syscall.Statfs(dir, &st)` (stdlib, cross-platform via build tags).
   - macOS: `st.Bavail` (available to non-superuser) × `st.Bsize` → bytes
     available.
   - Linux: `st.Bavail` × `st.Bsize` → bytes available.
   - Note: Use `syscall` (not `golang.org/x/sys/unix`) to avoid adding new
     imports; `syscall.Statfs_t` exists on both macOS and Linux.
4. Convert to MB: `availableMB := bytes / (1024 * 1024)`.
5. Threshold: 100 MB.
   - ≥ 100 MB: `Status = "ok"`,
     `Detail = fmt.Sprintf("%d MB available", availableMB)`.
   - < 100 MB: `Status = "fail"`,
     `Detail = fmt.Sprintf("%d MB available, need ≥ 100 MB", availableMB)`.

```
→ ok:    checkResult{Name: "Disk space", Status: "ok",   Detail: "512 MB available"}
→ low:   checkResult{Name: "Disk space", Status: "fail", Detail: "42 MB available, need ≥ 100 MB"}
```

**Implementation note for `syscall.Statfs_t`:**

```go
import "syscall"

var st syscall.Statfs_t
if err := syscall.Statfs(filepath.Dir(dbPath), &st); err != nil {
    // fail
}
availableBytes := uint64(st.Bavail) * uint64(st.Bsize)
availableMB := availableBytes / (1024 * 1024)
```

This works on both macOS and Linux (Go's `syscall` package abstracts the structs
via build tags).

### Check 5: Upstream connectivity — `checkUpstreams(routes []proxy.RouteConfig) []checkResult`

**Only executed when `--check-connectivity` is set.** Otherwise, a single
synthetic result:

```
checkResult{Name: "Upstreams", Status: "skip", Detail: "skipped (use --check-connectivity)"}
```

When enabled, for each unique `UpstreamHost` across loaded routes:

1. **Skip empty hosts** (local capture routes with `upstream_host == ""`).
2. **Deduplicate** hosts into a `map[string]struct{}` (multiple routes may
   reference the same host).
3. For each host, attempt `net.DialTimeout("tcp", host+":443", 3*time.Second)`.
4. On success: `Status = "ok"`,
   `Detail = fmt.Sprintf("%s:443 reachable", host)`.
5. On failure: `Status = "fail"`,
   `Detail = fmt.Sprintf("%s:443 unreachable: %v", host, err)`.
   - `net.DialTimeout` errors already include useful messages:
     `connection refused`, `i/o timeout`, `no such host`.

Each unique upstream host produces its own `checkResult`:

```
→ checkResult{Name: "Upstream api.openai.com",    Status: "ok",   Detail: "api.openai.com:443 reachable"}
→ checkResult{Name: "Upstream api.anthropic.com",  Status: "fail", Detail: "api.anthropic.com:443 unreachable: i/o timeout"}
```

When `--check-connectivity` is set but there are **zero** hosts to check (all
routes have empty `upstream_host` / are local), skip the section entirely and
emit:

```
checkResult{Name: "Upstreams", Status: "skip", Detail: "no remote hosts to check"}
```

---

## 3. Output Format

### Success (color enabled)

```
llm-proxy doctor
────────────────────────────────────────────────
 ✓ Version:         llm-proxy 0.1.0-dev
 ✓ Routes config:   valid (3 routes) [/etc/llm-proxy/routes.json]
 ✓ Database:        writable [/Users/alice/.local/share/llm-proxy/store.db]
 ✓ Disk space:      OK (512 MB available)
 ✓ Upstreams:       skipped (use --check-connectivity)
────────────────────────────────────────────────
 All checks passed.
```

### Failure (color enabled)

```
llm-proxy doctor
────────────────────────────────────────────────
 ✓ Version:         llm-proxy 0.1.0-dev
 ✗ Routes config:   route "chat" (/v1/chat/completions): upstream_host is required


 1 check failed.
```

### No-color / piped / NO_COLOR

Uses `OK` / `FAIL` / `SKIP` instead of `✓` / `✗` / `-`:

```
llm-proxy doctor
────────────────────────────────────────────────
 OK  Version:         llm-proxy 0.1.0-dev
 OK  Routes config:   valid (3 routes) [/etc/llm-proxy/routes.json]
 FAIL Database:        cannot open database: read-only file system
 SKIP Upstreams:       skipped (use --check-connectivity)
────────────────────────────────────────────────
 1 check failed.
```

### Formatting Rules

- **Header**: `llm-proxy doctor` on its own line, followed by a `─` separator
  line (width 48 chars).
- **Status column**: Each check is a single line. Status indicator (`✓`/`✗`/`-`
  or `OK`/`FAIL`/`SKIP`) + space + left-aligned check name (padded to align
  details) + detail text.
- **Padding**: The name column is padded to `"Disk space"` width (12 chars) +
  colon. Use `fmt.Fprintf(w, " %s  %-*s %s\n", mark, nameWidth, name, detail)`.
- **Path display**: Wrap config and DB paths in square brackets: `[path]`.
- **Footer** after the separator:
  - `"All checks passed."` when all pass.
  - `"<N> check failed."` / `"<N> checks failed."` for 1+ failures.

### Column Width Calculation

The Name column values are:

- `"Version:"` → 8 chars
- `"Routes config:"` → 14 chars
- `"Database:"` → 10 chars
- `"Disk space:"` → 12 chars
- `"Upstreams:"` → 12 chars
- `"Upstream <host>:"` → variable → use max of 14 or len(host)+10

For simplicity, use a **fixed name width of 18** (enough for `"Upstreams:"` with
margin). When checking individual upstream hosts, use `"Upstream <host>:"` which
could be longer; pad to a consistent `nameWidth` calculated as
`max(18, maxLen(names))`.

---

## 4. Implementation Details

### New File: `internal/cli/doctor.go`

#### Types

```go
type checkResult struct {
    Name   string // "Routes config", "Database", etc.
    Status string // "ok", "fail", "skip"
    Detail string // human-readable detail after the name
}
```

#### Functions

```go
// runDoctor is the main entry point called from root.go's switch.
func runDoctor(args []string, stdout, stderr io.Writer) int
```

Responsibilities of `runDoctor`:

1. Parse flags (`--routes-config`, `--db`, `--check-connectivity`,
   `--no-color`).
2. Validate `--routes-config` is set (else print error to stderr, return 2).
3. Determine color mode via `resolveColorMode(stdout, noColorFlag)`.
4. Execute checks sequentially:
   - `checkVersion()`
   - `checkRoutesConfig(routesConfig)` — save `*proxy.ProxyConfig` for upstream
     check
   - `checkDatabase(dbPath)`
   - `checkDiskSpace(dbPath)`
   - `checkUpstreams(cfg.Routes)` if `--check-connectivity`, else skip
5. Call `printDoctorResults(stdout, results, useColor)`.
6. Return 0 or 1 based on any `Status == "fail"`.

#### Individual Check Functions

```go
func checkVersion() checkResult
func checkRoutesConfig(path string) (*proxy.ProxyConfig, checkResult)
func checkDatabase(path string) checkResult
func checkDiskSpace(dbPath string) checkResult
func checkUpstreams(routes []proxy.RouteConfig) []checkResult
```

Note: `checkRoutesConfig` returns the parsed `*ProxyConfig` so that
`checkUpstreams` can iterate the hosts.

#### Print Function

```go
func printDoctorResults(w io.Writer, results []checkResult, useColor bool)
```

Prints:

1. Header: `"llm-proxy doctor\n"` + separator line
2. Each result line with appropriate mark
3. Separator line
4. Summary: `"All checks passed."` or `"<N> check(s) failed."`

#### Color Resolution

```go
func resolveColorMode(stdout io.Writer, noColorFlag bool) bool {
    // 1. If --no-color flag is set → false
    if noColorFlag {
        return false
    }
    // 2. If NO_COLOR env is set (to any non-empty value) → false
    if os.Getenv("NO_COLOR") != "" {
        return false
    }
    // 3. If TERM=dumb → false
    if os.Getenv("TERM") == "dumb" {
        return false
    }
    // 4. If stdout is not a terminal → false
    if !log.IsTerminal(stdout) {
        return false
    }
    // Otherwise → true
    return true
}
```

This follows the same pattern already used in `internal/log/log.go:71`
(`NewWriterWithFormat`).

#### Mark Function

```go
func checkMark(status string, useColor bool) string {
    if !useColor {
        switch status {
        case "ok":
            return "OK  "
        case "fail":
            return "FAIL"
        default:
            return "SKIP"
        }
    }
    switch status {
    case "ok":
        return "✓"
    case "fail":
        return "✗"
    default:
        return "–"  // U+2013 em-dash (or use "-")
    }
}
```

---

## 5. Registration in `root.go`

### Changes to `root.go`

1. Add `doc: "Run diagnostics and checks"` to help text, or add the new command
   line.

2. In the `switch args[0]` block inside `Run()`:

   ```go
   case "doctor":
       return runDoctor(args[1:], stdout, stderr)
   ```

3. In `printUsage()`, add to the "Usage:" block:

   ```
   llm-proxy doctor --routes-config path [--db path] [--check-connectivity] [--no-color]
   ```

4. In `printUsage()`, add to the "Commands:" block:
   ```
   doctor            Run environment and configuration diagnostics.
   ```

---

## 6. Testing Strategy

All tests in `internal/cli/cli_test.go`, following the existing pattern
(`bytes.Buffer` for stdout/stderr, `Run()` calls, exit code checks,
`strings.Contains` on output).

### Test 1: `TestDoctorCommand_ValidConfig`

```go
func TestDoctorCommand_ValidConfig(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    os.WriteFile(configPath, []byte(`{
        "routes": [
            {"path": "/_ping", "capture": "local"},
            {"path": "/chat", "upstream_host": "example.com", "capture": "usage"}
        ]
    }`), 0644)

    dbPath := filepath.Join(dir, "store.db")

    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", dbPath}, &stdout, &stderr)

    // In test, stdout is a Buffer (not a terminal), so --no-color behavior expected
    if code != 0 {
        t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
    }
    out := stdout.String()
    for _, want := range []string{"All checks passed", "Version", "Routes config", "Database", "Disk space", "Upstreams"} {
        if !strings.Contains(out, want) {
            t.Fatalf("output missing %q:\n%s", want, out)
        }
    }
}
```

### Test 2: `TestDoctorCommand_InvalidConfig`

```go
func TestDoctorCommand_InvalidConfig(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    os.WriteFile(configPath, []byte(`{
        "routes": [{"path": "", "capture": "usage"}]
    }`), 0644)

    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", filepath.Join(dir, "store.db")}, &stdout, &stderr)

    if code != 1 {
        t.Fatalf("expected exit code 1, got %d", code)
    }
    // The failure detail should contain the labeled error from validation
    if !strings.Contains(stdout.String(), "path is required") {
        t.Fatalf("output missing validation error:\n%s", stdout.String())
    }
    if !strings.Contains(stdout.String(), "1 check failed") {
        t.Fatalf("missing failure summary:\n%s", stdout.String())
    }
}
```

### Test 3: `TestDoctorCommand_MissingRoutesConfig`

```go
func TestDoctorCommand_MissingRoutesConfig(t *testing.T) {
    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--db", "/tmp/test.db"}, &stdout, &stderr)
    if code != 2 {
        t.Fatalf("expected exit code 2, got %d", code)
    }
    if !strings.Contains(stderr.String(), "--routes-config") {
        t.Fatalf("expected missing flag error, got stderr: %s", stderr.String())
    }
}
```

### Test 4: `TestDoctorCommand_NonExistentDB`

```go
func TestDoctorCommand_NonExistentDB(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    os.WriteFile(configPath, []byte(`{"routes":[{"path":"/_ping","capture":"local"}]}`), 0644)

    // DB path whose parent directory doesn't exist will cause a failure
    // Actually, store.Open creates the parent dir. So use a truly unwritable path:
    dbPath := "/proc/nonexistent/store.db"  // not writable on macOS/Linux

    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", dbPath}, &stdout, &stderr)

    // Database check should fail
    if code == 0 {
        t.Fatalf("expected non-zero exit code for unwritable DB path")
    }
    if !strings.Contains(stdout.String(), "FAIL") || !strings.Contains(stdout.String(), "Database") {
        // Or with color ✓/✗ depending on terminal detection
        t.Fatalf("expected DB failure in output:\n%s", stdout.String())
    }
}
```

**Note**: `store.Open` calls `os.MkdirAll(filepath.Dir(path))` — on Linux this
may create even `/proc/nonexistent`. Use a read-only directory pattern, or test
with a path like `/dev/null/store.db` where the "directory" is actually a file.

### Test 5: `TestDoctorCommand_CheckConnectivity_Reachable`

```go
func TestDoctorCommand_CheckConnectivity_Reachable(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    // Use localhost as a known-reachable host? No — use a listener.
    // Better: this test should be tagged //go:build integration or skipped in short mode.
    t.Skip("requires network access")

    os.WriteFile(configPath, []byte(`{
        "routes": [{"path":"/chat","upstream_host":"api.openai.com","capture":"usage"}]
    }`), 0644)

    dbPath := filepath.Join(dir, "store.db")
    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", dbPath, "--check-connectivity"}, &stdout, &stderr)
    if code != 0 {
        t.Fatalf("exit code = %d, output:\n%s\nstderr: %s", code, stdout.String(), stderr.String())
    }
    if !strings.Contains(stdout.String(), "api.openai.com") {
        t.Fatalf("expected upstream host in output:\n%s", stdout.String())
    }
}
```

### Test 6: `TestDoctorCommand_CheckConnectivity_Unreachable`

```go
func TestDoctorCommand_CheckConnectivity_Unreachable(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    // Use a hostname that definitely won't resolve or won't have :443 open
    os.WriteFile(configPath, []byte(`{
        "routes": [{"path":"/chat","upstream_host":"192.0.2.1","capture":"usage"}]
    }`), 0644)  // 192.0.2.1 is TEST-NET-1, guaranteed unreachable per RFC 5737

    dbPath := filepath.Join(dir, "store.db")
    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", dbPath, "--check-connectivity"}, &stdout, &stderr)

    if code != 1 {
        t.Fatalf("expected exit code 1, got %d", code)
    }
    if !strings.Contains(stdout.String(), "192.0.2.1") {
        t.Fatalf("expected unreachable host in output:\n%s", stdout.String())
    }
}
```

### Test 7: `TestDoctorCommand_NoColorFlag`

```go
func TestDoctorCommand_NoColorFlag(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    os.WriteFile(configPath, []byte(`{"routes":[{"path":"/_ping","capture":"local"}]}`), 0644)

    dbPath := filepath.Join(dir, "store.db")
    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", dbPath, "--no-color"}, &stdout, &stderr)

    if code != 0 {
        t.Fatalf("exit code = %d", code)
    }
    out := stdout.String()
    // Should use OK/FAIL/SKIP text, not Unicode symbols
    if strings.Contains(out, "✓") || strings.Contains(out, "✗") {
        t.Fatalf("no-color output should not contain Unicode check marks:\n%s", out)
    }
    if !strings.Contains(out, "OK") && !strings.Contains(out, "SKIP") {
        t.Fatalf("expected OK or SKIP labels:\n%s", out)
    }
}
```

### Test 8: `TestDoctorCommand_NoColorEnvVars`

Test `NO_COLOR=1` environment variable and `TERM=dumb` disable color:

```go
func TestDoctorCommand_NoColorEnv(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    os.WriteFile(configPath, []byte(`{"routes":[{"path":"/_ping","capture":"local"}]}`), 0644)

    t.Setenv("NO_COLOR", "1")
    dbPath := filepath.Join(dir, "store.db")
    var stdout bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", dbPath}, &stdout, &bytes.Buffer{})
    if code != 0 {
        t.Fatalf("exit code = %d", code)
    }
    if strings.Contains(stdout.String(), "✓") {
        t.Fatalf("NO_COLOR output should not contain Unicode check marks")
    }
}
```

### Test 9: `TestDoctorCommand_EmptyRoutes`

```go
func TestDoctorCommand_EmptyRoutes(t *testing.T) {
    dir := t.TempDir()
    configPath := filepath.Join(dir, "routes.json")
    os.WriteFile(configPath, []byte(`{"routes":[]}`), 0644)

    dbPath := filepath.Join(dir, "store.db")
    var stdout, stderr bytes.Buffer
    code := Run([]string{"doctor", "--routes-config", configPath, "--db", dbPath}, &stdout, &stderr)
    if code != 0 {
        t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
    }
    if !strings.Contains(stdout.String(), "valid (0 routes)") {
        t.Fatalf("expected 'valid (0 routes)' in output:\n%s", stdout.String())
    }
}
```

---

## 7. Edge Cases

| Case                                                           | Expected Behavior                                                                                                                                                                  |
| -------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Empty routes config** (`{"routes":[]}`)                      | Routes check passes: `"valid (0 routes)"`                                                                                                                                          |
| **Non-existent DB path (parent dir exists)**                   | `store.Open` creates the file & schema — passes as "writable"                                                                                                                      |
| **Non-existent DB parent dir**                                 | `store.Open` calls `MkdirAll` — passes unless filesystem is read-only                                                                                                              |
| **Truly unwritable path** (e.g., `/dev/null/store.db`)         | Database check fails with descriptive error                                                                                                                                        |
| **Disk space check on non-existent directory**                 | DB is always opened first; if `store.Open` succeeds, dir exists. Disk space checks `filepath.Dir(dbPath)` which will exist. If somehow missing → `"directory does not exist"` fail |
| **Upstream with empty `upstream_host`** (local capture routes) | Skipped: not added to deduplicated host list                                                                                                                                       |
| **Multiple routes with same `upstream_host`**                  | Deduplicated: checked once                                                                                                                                                         |
| **Connectivity check timeout** (3s exceeded)                   | `net.DialTimeout` returns `i/o timeout` error — printed as `"unreachable: i/o timeout"`                                                                                            |
| **DNS resolution failure**                                     | `net.DialTimeout` returns `no such host` — printed as `"unreachable: no such host"`                                                                                                |
| **`--routes-config` missing**                                  | Exit 2, error to stderr                                                                                                                                                            |
| **`--routes-config` file doesn't exist**                       | Routes check fails with `"read routes config: open ...: no such file or directory"`                                                                                                |
| **Config with JSON parse error**                               | Routes check fails with `"parse routes config: ..."`                                                                                                                               |
| **DB disk space: exactly 100 MB**                              | Passes (≥ 100 MB threshold)                                                                                                                                                        |
| **No `--check-connectivity`, all routes local**                | Upstreams section shows `"skipped (use --check-connectivity)"`                                                                                                                     |
| **Piped stdout** (not a TTY)                                   | Color disabled automatically via `log.IsTerminal`                                                                                                                                  |
| **`NO_COLOR` env set to empty string**                         | `""` is falsy → color enabled (follows spec convention that NO*COLOR must be set to \_something*)                                                                                  |

---

## 8. Dependencies

### Disk Space via `syscall.Statfs`

**Use:** `syscall.Statfs` (stdlib) — no new dependency required.

```go
import "syscall"

func checkDiskSpace(dbPath string) checkResult {
    dir := filepath.Dir(dbPath)
    if _, err := os.Stat(dir); os.IsNotExist(err) {
        return checkResult{Name: "Disk space", Status: "fail", Detail: fmt.Sprintf("directory does not exist: %s", dir)}
    }
    var st syscall.Statfs_t
    if err := syscall.Statfs(dir, &st); err != nil {
        return checkResult{Name: "Disk space", Status: "fail", Detail: err.Error()}
    }
    availableBytes := uint64(st.Bavail) * uint64(st.Bsize)
    availableMB := availableBytes / (1024 * 1024)
    if availableMB >= 100 {
        return checkResult{Name: "Disk space", Status: "ok", Detail: fmt.Sprintf("%d MB available", availableMB)}
    }
    return checkResult{Name: "Disk space", Status: "fail", Detail: fmt.Sprintf("%d MB available, need ≥ 100 MB", availableMB)}
}
```

> **Note:** `syscall.Statfs_t.Bavail` is `int64` on macOS and `int64` on Linux.
> The multiplication `uint64(st.Bavail) * uint64(st.Bsize)` must handle sign
> correctly. Use `int64` arithmetic, then convert:
> `availableMB := (st.Bavail * st.Bsize) / (1024 * 1024)`.

### No New Module Dependencies

- `syscall` — stdlib ✓
- `net` (`net.DialTimeout`) — stdlib ✓
- `log.IsTerminal` — existing internal package ✓
- `proxy.LoadConfig` — existing internal package ✓
- `store.Open`, `store.DefaultPath`, `store.FormatPath` — existing internal
  package ✓
- `golang.org/x/sys` — already in `go.mod` as transitive dep, but **not needed**
  for this implementation

---

## 9. File-by-File Change Summary

### New File: `internal/cli/doctor.go` (~130 lines)

```
Package cli

Imports: syscall, net, flag, fmt, io, os, path/filepath, strings, time
         llm-proxy/internal/log
         llm-proxy/internal/proxy
         llm-proxy/internal/store

Types:
- checkResult{Name, Status, Detail string}

Functions:
- runDoctor(args []string, stdout, stderr io.Writer) int
- checkVersion() checkResult
- checkRoutesConfig(path string) (*proxy.ProxyConfig, checkResult)
- checkDatabase(path string) checkResult
- checkDiskSpace(dbPath string) checkResult
- checkUpstreams(routes []proxy.RouteConfig) []checkResult
- printDoctorResults(w io.Writer, results []checkResult, useColor bool)
- resolveColorMode(stdout io.Writer, noColorFlag bool) bool
- checkMark(status string, useColor bool) string
```

### Modified File: `internal/cli/root.go` (~5 lines changed)

```diff
  case "validate":
      return runValidate(args[1:], stdout, stderr)
+ case "doctor":
+     return runDoctor(args[1:], stdout, stderr)

  // In printUsage():
+ llm-proxy doctor --routes-config path [--db path] [--check-connectivity] [--no-color]

  // In Commands section:
+ doctor            Run environment and configuration diagnostics.
```

### New Tests in `internal/cli/cli_test.go` (~150 lines)

- `TestDoctorCommand_ValidConfig`
- `TestDoctorCommand_InvalidConfig`
- `TestDoctorCommand_MissingRoutesConfig`
- `TestDoctorCommand_NonExistentDB`
- `TestDoctorCommand_CheckConnectivity_Reachable` (integration, skippable)
- `TestDoctorCommand_CheckConnectivity_Unreachable`
- `TestDoctorCommand_NoColorFlag`
- `TestDoctorCommand_NoColorEnv`
- `TestDoctorCommand_EmptyRoutes`

---

## 10. Post-Implementation Checklist

- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] `go build ./...` succeeds
- [ ] Manual test:
      `llm-proxy doctor --routes-config ./examples/routes/basic.json` produces
      expected output
- [ ] Manual test:
      `llm-proxy doctor --routes-config ./examples/routes/basic.json --check-connectivity`
      probes upstreams
- [ ] Manual test:
      `llm-proxy doctor --routes-config ./examples/routes/basic.json --no-color`
      uses OK/FAIL
- [ ] Manual test: pipe to `cat` disables color automatically
- [ ] Commit: `feat(cli): add doctor command for environment diagnostics`

---

## 11. Implementation Order (within the chunk)

1. **Define types and helpers** — `checkResult`, `checkMark`, `resolveColorMode`
2. **Implement `runDoctor`** with flag parsing and sequential check
   orchestration
3. **Implement each check function** one at a time:
   - `checkVersion` (trivial, always pass)
   - `checkRoutesConfig` (reuses existing `proxy.LoadConfig`)
   - `checkDatabase` (reuses existing `store.Open`)
   - `checkDiskSpace` (new code, `syscall.Statfs`)
   - `checkUpstreams` (new code, `net.DialTimeout`)
4. **Implement `printDoctorResults`** — header, lines, footer, column alignment
5. **Register in `root.go`** — switch case + help text
6. **Write tests** — unit tests for each check function + integration tests via
   `Run()`
7. **Verify** — `go test`, `go vet`, `go build`, manual smoke test
8. **Commit**
