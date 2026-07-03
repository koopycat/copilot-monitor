package log

import (
	"io"
	"os"

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
