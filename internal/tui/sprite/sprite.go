package sprite

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

// Sprite color palette (Catppuccin Mocha).
var (
	_  = lipgloss.Color("") // transparent (zero value)
	cR = lipgloss.Color("#F38BA8") // Red
	cB = lipgloss.Color("#89B4FA") // Blue
	cG = lipgloss.Color("#A6E3A1") // Green
	cY = lipgloss.Color("#F9E2AF") // Yellow
	cP = lipgloss.Color("#FAB387") // Peach
	cK = lipgloss.Color("#F5C2E7") // Pink
	cC = lipgloss.Color("#94E2D5") // Cyan / Teal
	cW = lipgloss.Color("#CDD6F4") // White
	cD = lipgloss.Color("#45475A") // Dark (surface)
	cV = lipgloss.Color("#CBA6F7") // Violet / Mauve
	cL = lipgloss.Color("#A6E3A1") // Lime (same as green)
	cS = lipgloss.Color("#585B70") // Subtle dark
)

// px is a shorthand type for an 8×8 pixel grid.
// Empty string = transparent pixel.
type px = [8][8]lipgloss.Color

// frame holds one animation frame as an 8×8 pixel grid.
type frame struct {
	Pixels px
}

// actionFrames maps each Action to its animation frames.
var actionFrames = map[Action][]frame{
	Idle: {
		{Pixels: spriteIdle1},
		{Pixels: spriteIdle2},
	},
	WalkLeft: {
		{Pixels: spriteWalkL1},
		{Pixels: spriteWalkL2},
	},
	WalkRight: {
		{Pixels: spriteWalkR1},
		{Pixels: spriteWalkR2},
	},
	Jump: {
		{Pixels: spriteJump1},
		{Pixels: spriteJump2},
	},
	Climb: {
		{Pixels: spriteClimb1},
		{Pixels: spriteClimb2},
	},
	Fall: {
		{Pixels: spriteFall1},
		{Pixels: spriteFall2},
	},
}

// ---------------------------------------------------------------------------
// Sprite pixel data — little green alien guy
// ---------------------------------------------------------------------------
var (
	// Shorthand aliases for this sprite
	g = cG // body
	w = cW // eyes
	d = cD // dark (pupils/mouth)

	spriteIdle1 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, w, d, w, d, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, g, "", "", g, g, ""},
		{"", g, g, "", "", g, g, ""},
	}
	spriteIdle2 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, d, w, d, w, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, g, "", "", g, g, ""},
		{"", g, g, "", "", g, g, ""},
	}

	spriteWalkL1 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, w, d, w, d, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, g, "", g, g, "", ""},
		{"", g, "", "", g, "", "", ""},
	}
	spriteWalkL2 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, w, d, w, d, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, "", g, g, ""},
		{"", "", "", g, "", g, "", ""},
	}

	spriteWalkR1 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, d, w, d, w, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, "", g, g, ""},
		{"", "", "", g, "", g, "", ""},
	}
	spriteWalkR2 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, d, w, d, w, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, g, "", g, g, "", ""},
		{"", g, "", "", g, "", "", ""},
	}

	spriteJump1 = px{
		{"", g, "", "", "", "", g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, w, d, w, d, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, g, "", "", g, g, ""},
	}
	spriteJump2 = px{
		{"", g, "", "", "", "", g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, d, w, d, w, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, "", "", g, "", ""},
	}

	spriteClimb1 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, w, d, w, d, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", "", g, "", "", g, "", ""},
		{"", g, "", "", "", "", g, ""},
	}
	spriteClimb2 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, d, w, d, w, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, "", "", "", "", g, ""},
		{"", "", g, "", "", g, "", ""},
	}

	spriteFall1 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, d, w, d, w, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, "", "", "", "", g, ""},
		{"", g, "", "", "", "", g, ""},
	}
	spriteFall2 = px{
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", g, w, d, w, d, g, ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, g, g, g, "", ""},
		{"", g, g, g, g, g, g, ""},
		{"", "", g, "", "", g, "", ""},
		{"", "", "", "", "", "", "", ""},
	}
)

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

// SpriteWidth is the width of the sprite in terminal columns.
const SpriteWidth = 8

// SpriteHeight is the height of the sprite in terminal rows.
// 8 pixel rows / 2 pixels per half-block = 4 rows.
const SpriteHeight = 4

// Frames returns the styled lines for the current animation frame.
// Each 8×8 pixel grid is rendered as 4 lines of 8 half-block characters,
// where each character encodes two vertical pixels using ▀ with fg/bg colors.
func (s *Sprite) Frames() []string {
	frames := actionFrames[s.Action]
	if len(frames) == 0 {
		frames = actionFrames[Idle]
	}
	idx := s.Frame % len(frames)
	f := frames[idx]

	result := make([]string, SpriteHeight)
	for row := 0; row < SpriteHeight; row++ {
		topRow := row * 2
		botRow := row*2 + 1
		var b strings.Builder
		for col := 0; col < SpriteWidth; col++ {
			top := f.Pixels[topRow][col]
			bot := f.Pixels[botRow][col]
			if top == "" && bot == "" {
				// Both transparent — output a space.
				b.WriteRune(' ')
			} else if top != "" && bot != "" {
				// Both pixels filled — ▀ with fg=top, bg=bottom.
				style := lipgloss.NewStyle().
					Foreground(top).
					Background(bot)
				b.WriteString(style.Render("▀"))
			} else if top != "" {
				// Only top pixel — ▀ with fg=top, no bg.
				style := lipgloss.NewStyle().Foreground(top)
				b.WriteString(style.Render("▀"))
			} else {
				// Only bottom pixel — ▄ with fg=bottom, no bg.
				style := lipgloss.NewStyle().Foreground(bot)
				b.WriteString(style.Render("▄"))
			}
		}
		result[row] = b.String()
	}
	return result
}

// Width returns the width of the sprite in terminal columns.
func (s *Sprite) Width() int {
	return SpriteWidth
}

// Height returns the height of the sprite in terminal rows.
func (s *Sprite) Height() int {
	return SpriteHeight
}
