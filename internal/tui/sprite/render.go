package sprite

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Overlay composites the sprite onto the TUI output string.
// It only modifies the lines the sprite occupies. Transparent
// pixels (spaces in sprite frames) show through to the background.
// Semi-transparent cells (one pixel colored, one empty) show the
// sprite when over empty background and show the background when
// over content, avoiding terminal-default-background artifacts.
// Handles edge cases where the sprite is partially off-screen.
func Overlay(output string, s *Sprite) string {
	if s == nil {
		return output
	}

	lines := strings.Split(output, "\n")
	frames := s.Frames()
	pixels := s.CurrentPixels()
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

		topRow := i * 2
		botRow := i*2 + 1

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

			top := pixels[topRow][c]
			bot := pixels[botRow][c]
			fullyTransparent := c < len(raw) && raw[c] == ' '
			semiTransparent := (top == "") != (bot == "") // exactly one pixel empty

			if fullyTransparent {
				// Both pixels empty: show background.
				if screenCol < bgW {
					b.WriteString(ansi.Cut(bg, screenCol, screenCol+1))
				} else {
					b.WriteString(" ")
				}
			} else if semiTransparent {
				// One pixel colored, one empty. The half-block renders the
				// colored pixel correctly, but the empty half would show the
				// terminal's default background instead of the content behind.
				// Show the sprite cell only when the background is empty;
				// otherwise fall through to the background so the empty half
				// doesn't create a visible artifact.
				bgCell := ""
				if screenCol < bgW {
					bgCell = ansi.Cut(bg, screenCol, screenCol+1)
				}
				bgRune := strings.TrimSpace(ansi.Strip(bgCell))
				if bgRune == "" {
					// Background is empty — safe to show the half-block;
					// the empty half blends with the terminal background.
					b.WriteString(ansi.Cut(styledFrame, c, c+1))
				} else {
					// Background has content — show it instead to avoid
					// a dark rectangle from the terminal default bg.
					b.WriteString(bgCell)
				}
			} else {
				// Both pixels colored: fully opaque, always show sprite.
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
