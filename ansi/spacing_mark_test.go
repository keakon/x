package ansi

import (
	"strings"
	"testing"

	upstream "github.com/charmbracelet/x/ansi"
)

func TestSpacingMarksWrapAndCut(t *testing.T) {
	for _, cluster := range []string{"a\u093e", "a\u093e\u093e", " \u093e", "-\u093e", "\u092a\u0949", "\u0e19\u0e49\u0e33", "\u0e99\u0eb3"} {
		if got := Cut(cluster+"x", 0, 1); got != "" {
			t.Fatalf("Cut(%q, 0, 1) = %q", cluster+"x", got)
		}
		if got := Cut(cluster+"x", 0, 2); got != cluster {
			t.Fatalf("Cut(%q, 0, 2) = %q", cluster+"x", got)
		}
		if got := Cut(cluster+"x", 2, 3); got != "x" {
			t.Fatalf("Cut(%q, 2, 3) = %q", cluster+"x", got)
		}
		for _, wrap := range []func(string, int) string{
			func(s string, n int) string { return Hardwrap(s, n, true) },
			func(s string, n int) string { return Wrap(s, n, "") },
		} {
			original := strings.Repeat(cluster, 4)
			wrapped := wrap(original, 5)
			if strings.ReplaceAll(wrapped, "\n", "") != original {
				t.Fatalf("wrap changed grapheme contents: %q", wrapped)
			}
			for _, line := range strings.Split(wrapped, "\n") {
				if StringWidth(line) > 5 {
					t.Fatalf("wrapped line too wide: %q", line)
				}
			}
		}
		if got := Wordwrap("x "+cluster+" y", 20, ""); got != "x "+cluster+" y" {
			t.Fatalf("Wordwrap changed a cluster: %q", got)
		}
	}
}

// colorprofile returns these shared palette types; their identity must survive
// a trip through this ANSI fork so palette indices remain palette indices.
func TestPaletteTypeIdentity(t *testing.T) {
	var basic BasicColor = upstream.BasicColor(3)
	var indexed IndexedColor = upstream.IndexedColor(200)
	if got := Convert16(basic); got != basic {
		t.Fatal("basic palette index changed")
	}
	if got := Convert256(indexed); got != indexed {
		t.Fatal("indexed palette index changed")
	}
}
