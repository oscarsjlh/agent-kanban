package tui

import (
	"fmt"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders markdown styled for the terminal, falling back to
// the raw text if glamour cannot render it.
func renderMarkdown(width int, src string) string {
	if width <= 0 {
		return src
	}
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(width))
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}

// trunc clips s to at most n runes, appending an ellipsis when clipped.
func trunc(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	if n <= 1 {
		return string(rs[:n])
	}
	return string(rs[:n-1]) + "…"
}

// padRight pads s with spaces to width n (rune-aware).
func padRight(s string, n int) string {
	rs := []rune(s)
	if len(rs) >= n {
		return string(rs[:n])
	}
	out := make([]rune, 0, n)
	out = append(out, rs...)
	for len(out) < n {
		out = append(out, ' ')
	}
	return string(out)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
