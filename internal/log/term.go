package log

import (
	"io"
	"os"
	"strconv"

	"golang.org/x/term"
)

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// IsTerminal reports whether w is a terminal (TTY).
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f)
}

// TerminalWidth returns the width of the terminal, or a sensible default.
// Checks the COLUMNS environment variable first, then the TIOCGWINSZ ioctl
// if f is a terminal, and falls back to 80.
func TerminalWidth(f *os.File) int {
	if s := os.Getenv("COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	if f != nil && isTerminal(f) {
		w, _, err := term.GetSize(int(f.Fd()))
		if err == nil && w > 0 {
			return w
		}
	}
	return 80
}
