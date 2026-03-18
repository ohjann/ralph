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
	headColor = lipgloss.Color("#F9E2AF") // Yellow (head)
	bodyColor = lipgloss.Color("#F9845C") // Peach (body)

	headStyle = lipgloss.NewStyle().Foreground(headColor)
	bodyStyle = lipgloss.NewStyle().Foreground(bodyColor)
)

// frame holds the two lines of a single animation frame.
type frame struct {
	Head string // top line (3 chars wide)
	Body string // bottom line (3 chars wide)
}

// actionFrames maps each Action to its animation frames.
var actionFrames = map[Action][]frame{
	Idle: {
		{Head: " o ", Body: "/|\\"},
		{Head: " o ", Body: "/|\\"},
	},
	WalkLeft: {
		{Head: " o ", Body: "/|\\"},
		{Head: " o ", Body: "|/\\"},
	},
	WalkRight: {
		{Head: " o ", Body: "/|\\"},
		{Head: " o ", Body: "/\\|"},
	},
	Jump: {
		{Head: "\\o/", Body: " | "},
		{Head: "\\o/", Body: "/ \\"},
	},
	Climb: {
		{Head: " o ", Body: "/|\\"},
		{Head: " o ", Body: "\\|/"},
	},
	Fall: {
		{Head: " o ", Body: "\\|/"},
		{Head: " o ", Body: " | "},
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
