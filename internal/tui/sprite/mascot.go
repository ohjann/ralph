package sprite

// Mascot bundles Sprites, World, and AIs into a single facade
// that the TUI model can drive with simple method calls.
// It supports multiple sprites (one per worker).
type Mascot struct {
	Spr         *Sprite   // primary sprite (first worker / interactive target)
	Sprites     []*Sprite // all sprites (including primary)
	AIs         []*AI     // one AI per sprite
	World       World
	Interactive bool    // when true, AI is paused and user controls the primary sprite
	lastX       float64 // previous X for detecting movement stopped in interactive mode
	idleTicks   int     // ticks since last movement in interactive mode
}

// NewMascot creates a Mascot with one sprite per worker.
// The sprites will be placed on platforms once the world is built via Resize.
func NewMascot(workers int) *Mascot {
	if workers < 1 {
		workers = 1
	}
	sprites := make([]*Sprite, workers)
	ais := make([]*AI, workers)
	for i := 0; i < workers; i++ {
		sprites[i] = NewSprite(float64(10+i*12), 9999)
		ais[i] = NewAI(int64(42 + i*17))
	}
	return &Mascot{
		Spr:     sprites[0],
		Sprites: sprites,
		AIs:     ais,
	}
}

// Resize rebuilds the world geometry from the given layout parameters
// and repositions all sprites onto the closest valid platform.
func (m *Mascot) Resize(lp LayoutParams) {
	m.World = BuildWorld(lp)
	if len(m.World.Platforms) == 0 {
		return
	}

	for _, spr := range m.Sprites {
		// Find the closest platform to the sprite's current Y position.
		// On first resize (Y=9999), this naturally picks the bottom platform.
		bestIdx := 0
		bestDist := int(^uint(0) >> 1)
		iy := int(spr.Y)
		for i, p := range m.World.Platforms {
			dy := iy - (p.Y - spr.Height() + 1)
			if dy < 0 {
				dy = -dy
			}
			if dy < bestDist {
				bestDist = dy
				bestIdx = i
			}
		}

		p := m.World.Platforms[bestIdx]
		ix := int(spr.X)

		// Clamp X to stay within the platform.
		if ix < p.X1 {
			spr.X = float64(p.X1)
		} else if ix+spr.Width()-1 > p.X2 {
			spr.X = float64(p.X2 - spr.Width() + 1)
		}

		// Snap Y to the platform surface.
		spr.Y = float64(p.Y - spr.Height() + 1)
		spr.OnGround = true
		spr.OnLadder = false
		spr.VelY = 0
	}
}

// Tick advances all sprites by one frame (AI + physics).
// It skips ticking if the world has not been initialized yet (no platforms).
func (m *Mascot) Tick() {
	if len(m.World.Platforms) == 0 {
		return
	}
	for i, spr := range m.Sprites {
		if i == 0 && m.Interactive {
			// Primary sprite in interactive mode: detect idle from key release.
			if spr.OnGround && (spr.Action == WalkLeft || spr.Action == WalkRight) {
				if spr.X == m.lastX {
					m.idleTicks++
					if m.idleTicks > 5 {
						spr.Action = Idle
						spr.Frame = 0
						spr.FrameTick = 0
					}
				} else {
					m.idleTicks = 0
				}
			}
			m.lastX = spr.X
		} else {
			m.AIs[i].Tick(spr, &m.World)
		}
		spr.Update(&m.World)
	}
}

// Overlay composites all sprites onto the given TUI output string.
func (m *Mascot) Overlay(output string) string {
	for _, spr := range m.Sprites {
		output = Overlay(output, spr)
	}
	return output
}
