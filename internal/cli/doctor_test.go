package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"copilot-monitoring/internal/store"
)

func TestDoctorHealthyLocalSetup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	proxy := doctorHealthServer(t, "/_health")
	defer proxy.Close()
	dashboard := doctorHealthServer(t, "/api/health")
	defer dashboard.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"doctor",
		"--db", dbPath,
		"--proxy-url", proxy.URL,
		"--dashboard-url", dashboard.URL,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"PASS  database", "PASS  proxy", "PASS  dashboard", "Result: ready"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDoctorJSON(t *testing.T) {
	proxy := doctorHealthServer(t, "/_health")
	defer proxy.Close()
	dashboard := doctorHealthServer(t, "/api/health")
	defer dashboard.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"doctor",
		"--db", filepath.Join(t.TempDir(), "missing", "store.db"),
		"--proxy-url", proxy.URL,
		"--dashboard-url", dashboard.URL,
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	var report doctorReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if !report.OK {
		t.Fatalf("report should be ok: %#v", report)
	}
	if len(report.Checks) != 5 {
		t.Fatalf("checks = %#v", report.Checks)
	}
	if report.Checks[0].Name != "database" || report.Checks[0].Status != "warn" {
		t.Fatalf("database check = %#v", report.Checks[0])
	}
}

func TestDoctorUnavailableProxyFails(t *testing.T) {
	proxy := doctorHealthServer(t, "/_health")
	proxyURL := proxy.URL
	proxy.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"doctor",
		"--db", filepath.Join(t.TempDir(), "store.db"),
		"--proxy-url", proxyURL,
		"--skip-dashboard",
		"--timeout", "50ms",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL  proxy") || !strings.Contains(stdout.String(), "Result: needs attention") {
		t.Fatalf("unexpected output:\n%s", stdout.String())
	}
}

func TestDoctorMissingDatabaseDoesNotCreateIt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "not-created", "store.db")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"doctor",
		"--db", dbPath,
		"--skip-proxy",
		"--skip-dashboard",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("database should not be created, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); !os.IsNotExist(err) {
		t.Fatalf("database parent should not be created, stat error = %v", err)
	}
	if !strings.Contains(stdout.String(), "WARN  database") {
		t.Fatalf("unexpected output:\n%s", stdout.String())
	}
}

func TestDoctorInvalidDatabaseFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "not-a-db")
	if err := os.WriteFile(dbPath, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"doctor",
		"--db", dbPath,
		"--skip-proxy",
		"--skip-dashboard",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL  database") {
		t.Fatalf("unexpected output:\n%s", stdout.String())
	}
}

func TestDoctorUpstreamTCPCheck(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
		close(accepted)
	}()

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"doctor",
		"--db", filepath.Join(t.TempDir(), "store.db"),
		"--skip-proxy",
		"--skip-dashboard",
		"--upstream", listener.Addr().String(),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for upstream TCP check")
	}
	if !strings.Contains(stdout.String(), "PASS  upstream") {
		t.Fatalf("unexpected output:\n%s", stdout.String())
	}
}

func TestDoctorInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"doctor", "--timeout", "0"},
		{"doctor", "--proxy-url", "api.githubcopilot.com"},
		{"doctor", "--upstream", "https://api.githubcopilot.com"},
		{"doctor", "--upstream", "api.githubcopilot.com:not-a-port"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("Run(%q) exit code = %d, want 2; stderr = %s", args, code, stderr.String())
		}
	}
}

func doctorHealthServer(t *testing.T, wantPath string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
}

func TestDoctorHealthRejectsUnhealthyPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"error"}`))
	}))
	defer server.Close()

	check := checkDoctorHealth(&http.Client{Timeout: time.Second}, "proxy", server.URL, "fix it")
	if check.Status != "fail" {
		t.Fatalf("check = %#v", check)
	}
}

func TestDoctorDatabaseDoesNotUseStoreInitialization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "store.db")
	check := checkDoctorDatabase(path)
	if check.Status != "warn" {
		t.Fatalf("check = %#v", check)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("database should not be created, stat error = %v", err)
	}

	// Keep this assertion close to the direct helper test: a regular store open
	// would create the path, so a missing path proves doctor avoided it.
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("database parent should not be created, stat error = %v", err)
	}
}
