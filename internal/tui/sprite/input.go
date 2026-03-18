package sprite

import tea "github.com/charmbracelet/bubbletea"

// HandleKey processes a key press during interactive mode.
// It returns true if the key was consumed (i.e. the mascot handled it).
func HandleKey(key tea.KeyMsg, s *Sprite, w *World) bool {
	switch key.String() {
	case "left":
		s.Walk(-1, w)
		return true
	case "right":
		s.Walk(1, w)
		return true
	case "up":
		s.StartClimbOnLadder(-1, w)
		return true
	case "down":
		s.StartClimbOnLadder(1, w)
		return true
	case " ": // space
		s.Jump()
		return true
	}
	return false
}
