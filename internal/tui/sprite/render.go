package sprite

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Overlay composites the sprite onto the TUI output string.
// It only modifies the lines the sprite occupies. Transparent
// pixels (spaces in sprite frames) show through to the background.
// Handles edge cases where the sprite is partially off-screen.
func Overlay(output string, s *Sprite) string {
	if s == nil {
		return output
	}

	lines := strings.Split(output, "\n")
	frames := s.Frames()
	col := int(s.X)
	row := int(s.Y)
	w := s.Width()

	for i, styledFrame := range frames {
		y := row + i
		if y < 0 || y >= len(lines) {
			continue
		}

		raw := []rune(ansi.Strip(styledFrame))
		bg := lines[y]
		bgW := ansi.StringWidth(bg)

		var b strings.Builder

		// Prefix: background before sprite
		prefixEnd := col
		if prefixEnd < 0 {
			prefixEnd = 0
		}
		if prefixEnd > 0 {
			if prefixEnd <= bgW {
				b.WriteString(ansi.Cut(bg, 0, prefixEnd))
			} else {
				b.WriteString(bg)
				b.WriteString(strings.Repeat(" ", prefixEnd-bgW))
			}
		}

		// Sprite area with transparency
		for c := 0; c < w; c++ {
			screenCol := col + c
			if screenCol < 0 {
				continue
			}

			transparent := c < len(raw) && raw[c] == ' '
			if transparent {
				if screenCol < bgW {
					b.WriteString(ansi.Cut(bg, screenCol, screenCol+1))
				} else {
					b.WriteString(" ")
				}
			} else {
				b.WriteString(ansi.Cut(styledFrame, c, c+1))
			}
		}

		// Suffix: background after sprite
		suffixStart := col + w
		if suffixStart < bgW {
			b.WriteString(ansi.Cut(bg, suffixStart, bgW))
		}

		lines[y] = b.String()
	}

	return strings.Join(lines, "\n")
}
