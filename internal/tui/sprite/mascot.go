package sprite

// Mascot bundles a Sprite, World, and AI into a single facade
// that the TUI model can drive with simple method calls.
type Mascot struct {
	Spr   *Sprite
	World World
	AI    *AI
}

// NewMascot creates a Mascot with a sprite positioned on the first platform.
func NewMascot() *Mascot {
	return &Mascot{
		Spr: NewSprite(10, 1),
		AI:  NewAI(42),
	}
}

// Resize rebuilds the world geometry from the given layout parameters
// and repositions the sprite if necessary.
func (m *Mascot) Resize(lp LayoutParams) {
	m.World = BuildWorld(lp)
	// Place sprite on the first platform if it's out of bounds.
	if len(m.World.Platforms) > 0 {
		p := m.World.Platforms[0]
		ix, iy := int(m.Spr.X), int(m.Spr.Y)
		if ix < p.X1 || ix+m.Spr.Width()-1 > p.X2 || iy < 0 || iy >= m.World.Height {
			m.Spr.X = float64(p.X1 + (p.X2-p.X1)/2)
			m.Spr.Y = float64(p.Y - m.Spr.Height())
			m.Spr.OnGround = true
		}
	}
}

// Tick advances the mascot by one frame (AI + physics).
func (m *Mascot) Tick() {
	m.AI.Tick(m.Spr, &m.World)
	m.Spr.Update(&m.World)
}

// Overlay composites the sprite onto the given TUI output string.
func (m *Mascot) Overlay(output string) string {
	return Overlay(output, m.Spr)
}
