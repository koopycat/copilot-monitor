package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

func runConfigure(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("configure-vscode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:7733", "proxy listen address")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	printVSCodeSettings(stdout, *addr)
	return 0
}

func printVSCodeSettings(w io.Writer, addr string) {
	baseURL := "http://" + settingsAddr(addr) + "/copilot"
	fmt.Fprintf(w, "{\n")
	fmt.Fprintf(w, "  \"github.copilot.advanced\": {\n")
	fmt.Fprintf(w, "    \"debug.overrideCapiUrl\": %q\n", baseURL)
	fmt.Fprintf(w, "  }\n")
	fmt.Fprintf(w, "}\n")
}

func settingsAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}
