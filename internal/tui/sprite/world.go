package sprite

// Platform represents a horizontal surface the sprite can walk on.
type Platform struct {
	Y  int // row of the platform surface
	X1 int // left edge (inclusive)
	X2 int // right edge (inclusive)
}

// Ladder represents a vertical structure the sprite can climb.
type Ladder struct {
	X  int // column of the ladder
	Y1 int // top of the ladder (inclusive)
	Y2 int // bottom of the ladder (inclusive)
}

// World holds all walkable geometry derived from the TUI panel layout.
type World struct {
	Platforms []Platform
	Ladders   []Ladder
	Width     int
	Height    int
}

// LayoutParams captures the TUI layout measurements needed to derive
// platform and ladder positions. Field names mirror the calculations in
// model.go (WindowSizeMsg handler, lines 505-532).
type LayoutParams struct {
	Width        int  // terminal width
	Height       int  // terminal height
	HasStuckBar  bool // whether the stuck-alert status bar is visible
	HasHintInput bool // whether the hint input is active
}

// BuildWorld derives a World from the given TUI layout parameters.
// It produces three platforms (top panel border, middle border between top
// and bottom panels, bottom panel border) and ladders at panel edges.
func BuildWorld(lp LayoutParams) World {
	// --- replicate model.go layout math ---
	chrome := 4 // header(3) + footer(1)
	if lp.HasStuckBar {
		chrome++
	}
	if lp.HasHintInput {
		chrome += 3
	}
	available := lp.Height - chrome
	if available < 10 {
		available = 10
	}
	topHeight := available * 35 / 100
	if topHeight < 5 {
		topHeight = 5
	}
	claudeHeight := available - topHeight

	storiesWidth := lp.Width * 35 / 100
	// contextWidth := lp.Width - storiesWidth // used for ladder placement

	// The header occupies 3 rows (rows 0-2). Panel content starts at row 3.
	headerRows := 3

	// --- platform positions ---
	// Top border of top panels (the top edge of Stories/Context boxes).
	topBorderY := headerRows
	// Middle border: bottom of top panels / top of Claude panel.
	midBorderY := headerRows + topHeight
	// Bottom border of Claude panel.
	bottomBorderY := headerRows + topHeight + claudeHeight - 1

	platforms := []Platform{
		{Y: topBorderY, X1: 0, X2: lp.Width - 1},
		{Y: midBorderY, X1: 0, X2: lp.Width - 1},
		{Y: bottomBorderY, X1: 0, X2: lp.Width - 1},
	}

	// --- ladders ---
	// Ladders connect adjacent platforms at vertical panel edges.
	// Left edge of terminal.
	// Right edge of stories panel / left edge of context panel.
	// Right edge of terminal.
	ladderPositions := []int{
		0,                // left wall
		storiesWidth - 1, // right edge of stories panel
		storiesWidth,     // left edge of context panel
		lp.Width - 1,     // right wall
	}

	// Top ladders: connect top border to mid border.
	// Bottom ladders: connect mid border to bottom border.
	var ladders []Ladder
	for _, x := range ladderPositions {
		if x < 0 || x >= lp.Width {
			continue
		}
		ladders = append(ladders, Ladder{X: x, Y1: topBorderY, Y2: midBorderY})
		ladders = append(ladders, Ladder{X: x, Y1: midBorderY, Y2: bottomBorderY})
	}

	return World{
		Platforms: platforms,
		Ladders:   ladders,
		Width:     lp.Width,
		Height:    lp.Height,
	}
}

// PlatformAt returns the platform at position (x, y), if any.
func (w *World) PlatformAt(x, y int) *Platform {
	for i := range w.Platforms {
		p := &w.Platforms[i]
		if y == p.Y && x >= p.X1 && x <= p.X2 {
			return p
		}
	}
	return nil
}

// LadderAt returns the ladder at position (x, y), if any.
func (w *World) LadderAt(x, y int) *Ladder {
	for i := range w.Ladders {
		l := &w.Ladders[i]
		if x == l.X && y >= l.Y1 && y <= l.Y2 {
			return l
		}
	}
	return nil
}

// ClampPosition constrains an entity of size (w, h) so it stays within world
// bounds. The returned (x, y) is the top-left corner of the clamped bounding
// box.
func (world *World) ClampPosition(x, y, w, h int) (int, int) {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > world.Width {
		x = world.Width - w
	}
	if y+h > world.Height {
		y = world.Height - h
	}
	// Final safety: if the entity is larger than the world, pin to origin.
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}
