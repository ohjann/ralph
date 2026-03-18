package sprite

import "github.com/charmbracelet/lipgloss"

// Action represents the current action state of a sprite.
type Action int

const (
	Idle Action = iota
	WalkLeft
	WalkRight
	Jump
	Climb
	Fall
)

// Catppuccin Mocha palette colors for sprite rendering.
var (
	headColor = lipgloss.Color("#89B4FA") // Blue (robot head)
	bodyColor = lipgloss.Color("#74C7EC") // Sapphire (robot body)

	headStyle = lipgloss.NewStyle().Foreground(headColor)
	bodyStyle = lipgloss.NewStyle().Foreground(bodyColor)
)

// frame holds the two lines of a single animation frame.
type frame struct {
	Head string // top line (3 chars wide)
	Body string // bottom line (3 chars wide)
}

// actionFrames maps each Action to its animation frames.
// Robot sprite rendered in braille characters (6x8 pixel grid per 3x2 char frame).
var actionFrames = map[Action][]frame{
	Idle: {
		{Head: "⣯⣿⣽", Body: "⢾⠿⡷"},
		{Head: "⣯⣿⣽", Body: "⢾⠿⡷"},
	},
	WalkLeft: {
		{Head: "⣯⣿⣽", Body: "⢺⠟⣗"},
		{Head: "⣯⣿⣽", Body: "⡾⠛⣗"},
	},
	WalkRight: {
		{Head: "⣯⣿⣽", Body: "⣺⠻⡗"},
		{Head: "⣯⣿⣽", Body: "⣺⠛⢷"},
	},
	Jump: {
		{Head: "⣗⣿⣺", Body: "⢚⠿⡓"},
		{Head: "⣗⣿⣺", Body: "⠚⠿⠓"},
	},
	Climb: {
		{Head: "⣯⣿⣽", Body: "⢞⠿⡳"},
		{Head: "⣯⣿⣽", Body: "⡺⣛⢗"},
	},
	Fall: {
		{Head: "⣯⣿⣽", Body: "⡝⠛⢫"},
		{Head: "⣯⣿⣽", Body: "⡙⠛⢋"},
	},
}

// Sprite represents an animated character in the TUI world.
type Sprite struct {
	X         float64
	Y         float64
	VelY      float64
	Dir       int // -1 left, 0 neutral, 1 right
	Action    Action
	Frame     int
	FrameTick int
	OnGround  bool
	OnLadder  bool
}

// NewSprite creates a new Sprite at the given position.
func NewSprite(x, y float64) *Sprite {
	return &Sprite{
		X:        x,
		Y:        y,
		Dir:      1,
		Action:   Idle,
		OnGround: true,
	}
}

// Frames returns the styled ASCII art lines for the current action and frame.
func (s *Sprite) Frames() []string {
	frames := actionFrames[s.Action]
	if len(frames) == 0 {
		frames = actionFrames[Idle]
	}
	idx := s.Frame % len(frames)
	f := frames[idx]
	return []string{
		headStyle.Render(f.Head),
		bodyStyle.Render(f.Body),
	}
}

// Width returns the width of the sprite in characters.
func (s *Sprite) Width() int {
	return 3
}

// Height returns the height of the sprite in lines.
func (s *Sprite) Height() int {
	return 2
}
