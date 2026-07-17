package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"copilot-monitoring/internal/store"
)

const sqliteFileHeader = "SQLite format 3\x00"

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

type doctorReport struct {
	OK     bool          `json:"ok"`
	Checks []doctorCheck `json:"checks"`
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", store.DefaultPath(), "SQLite database path to inspect")
	proxyURL := fs.String("proxy-url", "http://127.0.0.1:7733", "local proxy base URL")
	dashboardURL := fs.String("dashboard-url", "http://127.0.0.1:7734", "local dashboard base URL")
	skipProxy := fs.Bool("skip-proxy", false, "skip the local proxy health check")
	skipDashboard := fs.Bool("skip-dashboard", false, "skip the local dashboard health check")
	upstream := fs.String("upstream", "", "optional upstream host[:port] to test with TCP")
	timeout := fs.Duration("timeout", 2*time.Second, "timeout for local and upstream checks")
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: doctor does not accept positional arguments")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "error: --timeout must be greater than zero")
		return 2
	}

	var proxyEndpoint, dashboardEndpoint string
	if !*skipProxy {
		base, err := parseDoctorBaseURL("--proxy-url", *proxyURL)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		proxyEndpoint = base + "/_health"
	}
	if !*skipDashboard {
		base, err := parseDoctorBaseURL("--dashboard-url", *dashboardURL)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		dashboardEndpoint = base + "/api/health"
	}

	var upstreamAddr string
	if *upstream != "" {
		var err error
		upstreamAddr, err = doctorUpstreamAddress(*upstream)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
	}

	report := doctorReport{Checks: []doctorCheck{checkDoctorDatabase(*dbPath)}}
	client := &http.Client{
		Timeout: *timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if *skipProxy {
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "proxy",
			Status: "skip",
			Detail: "skipped by --skip-proxy",
		})
	} else {
		report.Checks = append(report.Checks, checkDoctorHealth(client, "proxy", proxyEndpoint, "Start it with `copilot-monitor run --dashboard --upstream api.githubcopilot.com`."))
	}
	if *skipDashboard {
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "dashboard",
			Status: "skip",
			Detail: "skipped by --skip-dashboard",
		})
	} else {
		report.Checks = append(report.Checks, checkDoctorHealth(client, "dashboard", dashboardEndpoint, "Start it with `copilot-monitor serve` or add `--dashboard` to `copilot-monitor run`."))
	}
	if upstreamAddr == "" {
		report.Checks = append(report.Checks, doctorCheck{
			Name:   "upstream",
			Status: "skip",
			Detail: "not checked; pass --upstream host[:port] to test TCP reachability",
		})
	} else {
		report.Checks = append(report.Checks, checkDoctorUpstream(upstreamAddr, *timeout))
	}
	report.Checks = append(report.Checks, doctorCheck{
		Name:   "client_setup",
		Status: "info",
		Detail: "editor settings cannot be inspected by the proxy; for VS Code, confirm github.copilot.advanced.debug.overrideCapiUrl points to the local proxy and reload the window",
	})
	report.OK = !doctorHasFailures(report.Checks)

	if *jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "error: encoding json: %v\n", err)
			return 1
		}
	} else {
		renderDoctorReport(stdout, report)
	}
	if !report.OK {
		return 1
	}
	return 0
}

func parseDoctorBaseURL(flagName, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%s must be an absolute http(s) URL, got %q", flagName, raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%s must use http or https, got %q", flagName, u.Scheme)
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%s must be a base URL without credentials, path, query, or fragment", flagName)
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}

func doctorUpstreamAddress(upstream string) (string, error) {
	if strings.TrimSpace(upstream) != upstream || upstream == "" || strings.Contains(upstream, "://") || strings.ContainsAny(upstream, "/?#") {
		return "", fmt.Errorf("--upstream must be a host or host:port, got %q", upstream)
	}
	if host, port, err := net.SplitHostPort(upstream); err == nil {
		if host == "" || port == "" {
			return "", fmt.Errorf("--upstream must include a host and port, got %q", upstream)
		}
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portNumber == 0 {
			return "", fmt.Errorf("--upstream must use a port from 1 through 65535, got %q", upstream)
		}
		return upstream, nil
	}

	host := strings.Trim(upstream, "[]")
	if host == "" {
		return "", fmt.Errorf("--upstream must be a host or host:port, got %q", upstream)
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", fmt.Errorf("--upstream must be a host or host:port, got %q", upstream)
	}
	return net.JoinHostPort(host, "443"), nil
}

func checkDoctorDatabase(path string) doctorCheck {
	resolved := store.FormatPath(path)
	info, err := os.Stat(resolved)
	if errors.Is(err, os.ErrNotExist) {
		parent := filepath.Dir(resolved)
		if parentInfo, parentErr := os.Stat(parent); parentErr == nil && !parentInfo.IsDir() {
			return doctorCheck{
				Name:   "database",
				Status: "fail",
				Detail: fmt.Sprintf("parent path is not a directory: %s", parent),
				Fix:    "Choose a database path whose parent is a directory.",
			}
		}
		return doctorCheck{
			Name:   "database",
			Status: "warn",
			Detail: fmt.Sprintf("not created yet: %s", resolved),
			Fix:    "This is normal before the first capture; `run` creates it when it starts.",
		}
	}
	if err != nil {
		return doctorCheck{
			Name:   "database",
			Status: "fail",
			Detail: fmt.Sprintf("cannot inspect %s: %v", resolved, err),
			Fix:    "Check the database path and filesystem permissions.",
		}
	}
	if !info.Mode().IsRegular() {
		return doctorCheck{
			Name:   "database",
			Status: "fail",
			Detail: fmt.Sprintf("not a regular file: %s", resolved),
			Fix:    "Point --db at the Copilot Monitor SQLite database file.",
		}
	}

	f, err := os.Open(resolved)
	if err != nil {
		return doctorCheck{
			Name:   "database",
			Status: "fail",
			Detail: fmt.Sprintf("cannot read %s: %v", resolved, err),
			Fix:    "Check the database path and filesystem permissions.",
		}
	}
	defer f.Close()
	header := make([]byte, len(sqliteFileHeader))
	if _, err := io.ReadFull(f, header); err != nil || !bytes.Equal(header, []byte(sqliteFileHeader)) {
		return doctorCheck{
			Name:   "database",
			Status: "fail",
			Detail: fmt.Sprintf("not a readable SQLite database file: %s", resolved),
			Fix:    "Point --db at a valid Copilot Monitor SQLite database or move the invalid file aside.",
		}
	}
	return doctorCheck{
		Name:   "database",
		Status: "pass",
		Detail: fmt.Sprintf("readable SQLite database: %s", resolved),
	}
}

func checkDoctorHealth(client *http.Client, name, endpoint, fix string) doctorCheck {
	resp, err := client.Get(endpoint)
	if err != nil {
		return doctorCheck{
			Name:   name,
			Status: "fail",
			Detail: fmt.Sprintf("unreachable at %s: %v", endpoint, err),
			Fix:    fix,
		}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if err != nil {
		return doctorCheck{
			Name:   name,
			Status: "fail",
			Detail: fmt.Sprintf("could not read health response from %s: %v", endpoint, err),
			Fix:    fix,
		}
	}
	var payload struct {
		Status string `json:"status"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &payload) != nil || payload.Status != "ok" {
		return doctorCheck{
			Name:   name,
			Status: "fail",
			Detail: fmt.Sprintf("unhealthy response from %s: HTTP %d", endpoint, resp.StatusCode),
			Fix:    fix,
		}
	}
	return doctorCheck{
		Name:   name,
		Status: "pass",
		Detail: fmt.Sprintf("healthy at %s", endpoint),
	}
}

func checkDoctorUpstream(address string, timeout time.Duration) doctorCheck {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(context.Background(), "tcp", address)
	if err != nil {
		return doctorCheck{
			Name:   "upstream",
			Status: "fail",
			Detail: fmt.Sprintf("cannot connect to %s: %v", address, err),
			Fix:    "Check the upstream host, port, DNS, and network access.",
		}
	}
	_ = conn.Close()
	return doctorCheck{
		Name:   "upstream",
		Status: "pass",
		Detail: fmt.Sprintf("TCP reachable at %s", address),
	}
}

func doctorHasFailures(checks []doctorCheck) bool {
	for _, check := range checks {
		if check.Status == "fail" {
			return true
		}
	}
	return false
}

func renderDoctorReport(w io.Writer, report doctorReport) {
	fmt.Fprintln(w, "Copilot Monitor doctor")
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%s  %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Detail)
		if check.Fix != "" {
			fmt.Fprintf(w, "      %s\n", check.Fix)
		}
	}
	if report.OK {
		fmt.Fprintln(w, "Result: ready")
		return
	}
	fmt.Fprintln(w, "Result: needs attention")
}
