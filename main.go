// What-If Boat: an interactive version of the counterfactual "one subtraction" trick.
//
//  1. Aim with the mouse and click to sail. A hidden current pushes you around.
//  2. Aim a different heading. The game shows the plain model's guess (hollow)
//     and the trick's what-if answer (blue): actual landing + guess(new) - guess(old).
//  3. Click to replay the trip with the SAME current and see who was closer.
//
// Keys: Up/Down change trip length, C shows the current, R starts a new trip.
package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenW   = 960
	screenH   = 640
	pxPerUnit = 55.0 // one boat length = 55 pixels
	simSteps  = 200
)

var (
	colWater   = color.RGBA{18, 42, 66, 255}
	colCurrent = color.RGBA{60, 110, 140, 255}
	colTrail   = color.RGBA{200, 200, 200, 255}
	colGhost   = color.RGBA{120, 220, 160, 255}
	colGuess   = color.RGBA{190, 190, 190, 255}
	colAnswer  = color.RGBA{70, 150, 255, 255}
	colActual  = color.RGBA{255, 255, 255, 255}
	colAim     = color.RGBA{255, 210, 90, 255}
)

// ---------- the physics (same as the article's demo) ----------

type Vec struct{ X, Y float64 }

func (a Vec) Add(b Vec) Vec       { return Vec{a.X + b.X, a.Y + b.Y} }
func (a Vec) Sub(b Vec) Vec       { return Vec{a.X - b.X, a.Y - b.Y} }
func (a Vec) Scale(s float64) Vec { return Vec{a.X * s, a.Y * s} }
func (a Vec) Len() float64        { return math.Hypot(a.X, a.Y) }

// Current is the hidden luck: a steady push plus a swirl that depends on position.
type Current struct {
	Base  Vec
	Swirl float64
	Phase float64
}

func (c Current) At(p Vec) Vec {
	return Vec{
		c.Base.X + c.Swirl*math.Sin(p.Y+c.Phase),
		c.Base.Y + c.Swirl*math.Cos(p.X+c.Phase),
	}
}

func randomCurrent(r *rand.Rand) Current {
	return Current{
		Base:  Vec{r.NormFloat64(), r.NormFloat64()},
		Swirl: 1.5 * r.Float64(),
		Phase: 2 * math.Pi * r.Float64(),
	}
}

// sailPath is the real world: every point the boat passes through.
func sailPath(heading float64, c Current, h float64, steps int) []Vec {
	path := make([]Vec, 0, steps+1)
	p := Vec{}
	path = append(path, p)
	dt := h / float64(steps)
	engine := Vec{math.Cos(heading), math.Sin(heading)}
	for i := 0; i < steps; i++ {
		p = p.Add(engine.Add(c.At(p)).Scale(dt))
		path = append(path, p)
	}
	return path
}

// ---------- the world model: averages over currents, never sees yours ----------

type Model struct {
	h       float64
	guesses [360]Vec // one guess per degree of heading
}

func buildModel(h float64) *Model {
	r := rand.New(rand.NewSource(7))
	currents := make([]Current, 300)
	for i := range currents {
		currents[i] = randomCurrent(r)
	}
	m := &Model{h: h}
	for deg := 0; deg < 360; deg++ {
		heading := float64(deg) * math.Pi / 180
		sum := Vec{}
		for _, c := range currents {
			path := sailPath(heading, c, h, 60)
			sum = sum.Add(path[len(path)-1])
		}
		m.guesses[deg] = sum.Scale(1 / float64(len(currents)))
	}
	return m
}

// Predict blends the two nearest whole-degree guesses.
func (m *Model) Predict(heading float64) Vec {
	d := math.Mod(heading*180/math.Pi, 360)
	if d < 0 {
		d += 360
	}
	i := int(d) % 360
	j := (i + 1) % 360
	t := d - math.Floor(d)
	return m.guesses[i].Scale(1 - t).Add(m.guesses[j].Scale(t))
}

// ---------- the game ----------

type phase int

const (
	aiming    phase = iota // pick your heading
	sailing                // watch the real trip
	whatIf                 // pick the alternative heading
	replaying              // watch the alternative with the same current
	scored                 // see who was closer
)

type Game struct {
	rng   *rand.Rand
	h     float64
	model *Model

	current     Current
	showCurrent bool
	phase       phase
	frame       int

	did, instead float64
	realPath     []Vec
	ghostPath    []Vec
	guessDid     Vec
	guessInstead Vec
	answer       Vec

	trips                int
	plainMiss, trickMiss float64 // running totals in boat lengths
	lastPlain, lastTrick float64
}

func NewGame() *Game {
	g := &Game{rng: rand.New(rand.NewSource(42)), h: 1.5}
	g.model = buildModel(g.h)
	g.newTrip()
	return g
}

func (g *Game) newTrip() {
	g.current = randomCurrent(g.rng)
	g.phase = aiming
	g.realPath, g.ghostPath = nil, nil
}

// origin is where the boat starts on screen.
func origin() Vec { return Vec{screenW * 0.42, screenH * 0.55} }

func toScreen(p Vec) (float32, float32) {
	o := origin()
	return float32(o.X + p.X*pxPerUnit), float32(o.Y + p.Y*pxPerUnit)
}

func mouseHeading() float64 {
	mx, my := ebiten.CursorPosition()
	o := origin()
	return math.Atan2(float64(my)-o.Y, float64(mx)-o.X)
}

func (g *Game) Update() error {
	g.frame++
	clicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.showCurrent = !g.showCurrent
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.newTrip()
	}
	if g.phase == aiming {
		changed := false
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) && g.h < 3 {
			g.h += 0.25
			changed = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) && g.h > 0.25 {
			g.h -= 0.25
			changed = true
		}
		if changed {
			g.model = buildModel(g.h)
		}
	}

	switch g.phase {
	case aiming:
		if clicked {
			g.did = mouseHeading()
			g.realPath = sailPath(g.did, g.current, g.h, simSteps)
			g.guessDid = g.model.Predict(g.did)
			g.phase, g.frame = sailing, 0
		}
	case sailing:
		if g.frame >= 90 {
			g.phase = whatIf
		}
	case whatIf:
		g.instead = mouseHeading()
		g.guessInstead = g.model.Predict(g.instead)
		landed := g.realPath[len(g.realPath)-1]
		// The whole trick, one line:
		g.answer = landed.Add(g.guessInstead.Sub(g.guessDid))
		if clicked {
			g.ghostPath = sailPath(g.instead, g.current, g.h, simSteps)
			g.phase, g.frame = replaying, 0
		}
	case replaying:
		if g.frame >= 90 {
			truth := g.ghostPath[len(g.ghostPath)-1]
			g.lastPlain = g.guessInstead.Sub(truth).Len()
			g.lastTrick = g.answer.Sub(truth).Len()
			g.plainMiss += g.lastPlain
			g.trickMiss += g.lastTrick
			g.trips++
			g.phase = scored
		}
	case scored:
		if clicked {
			g.newTrip()
		}
	}
	return nil
}

func (g *Game) drawCurrent(screen *ebiten.Image) {
	o := origin()
	for sx := 30.0; sx < screenW; sx += 48 {
		for sy := 30.0; sy < screenH; sy += 48 {
			p := Vec{(sx - o.X) / pxPerUnit, (sy - o.Y) / pxPerUnit}
			v := g.current.At(p).Scale(12)
			vector.StrokeLine(screen, float32(sx), float32(sy), float32(sx+v.X), float32(sy+v.Y), 1.5, colCurrent, true)
			vector.FillCircle(screen, float32(sx+v.X), float32(sy+v.Y), 2, colCurrent, true)
		}
	}
}

// drawPath draws the first `upTo` points of a path, plus the boat at the end.
func drawPath(screen *ebiten.Image, path []Vec, upTo int, clr color.Color) {
	if upTo > len(path) {
		upTo = len(path)
	}
	for i := 1; i < upTo; i++ {
		x0, y0 := toScreen(path[i-1])
		x1, y1 := toScreen(path[i])
		vector.StrokeLine(screen, x0, y0, x1, y1, 2, clr, true)
	}
	if upTo > 0 {
		x, y := toScreen(path[upTo-1])
		vector.FillCircle(screen, x, y, 6, clr, true)
	}
}

func drawAim(screen *ebiten.Image, heading float64, clr color.Color) {
	ox, oy := toScreen(Vec{})
	ex := ox + float32(math.Cos(heading)*70)
	ey := oy + float32(math.Sin(heading)*70)
	vector.StrokeLine(screen, ox, oy, ex, ey, 2, clr, true)
}

func ring(screen *ebiten.Image, p Vec, clr color.Color) {
	x, y := toScreen(p)
	vector.StrokeCircle(screen, x, y, 7, 2, clr, true)
}

func dot(screen *ebiten.Image, p Vec, r float32, clr color.Color) {
	x, y := toScreen(p)
	vector.FillCircle(screen, x, y, r, clr, true)
}

func label(screen *ebiten.Image, p Vec, s string) {
	x, y := toScreen(p)
	ebitenutil.DebugPrintAt(screen, s, int(x)+10, int(y)-8)
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(colWater)
	if g.showCurrent {
		g.drawCurrent(screen)
	}
	ox, oy := toScreen(Vec{})
	vector.FillCircle(screen, ox, oy, 5, colGuess, true)

	steps := func() int { return 1 + g.frame*len(g.realPath)/90 }

	switch g.phase {
	case aiming:
		drawAim(screen, mouseHeading(), colAim)
		ring(screen, g.model.Predict(mouseHeading()), colGuess)
		label(screen, g.model.Predict(mouseHeading()), "model's guess")
	case sailing:
		drawPath(screen, g.realPath, steps(), colTrail)
	default:
		drawPath(screen, g.realPath, len(g.realPath), colTrail)
		landed := g.realPath[len(g.realPath)-1]
		ring(screen, g.guessDid, colGuess)
		label(screen, landed, "you landed here")

		if g.phase == whatIf {
			drawAim(screen, g.instead, colAim)
		}
		if g.phase == replaying {
			drawPath(screen, g.ghostPath, 1+g.frame*len(g.ghostPath)/90, colGhost)
		}
		if g.phase == scored {
			drawPath(screen, g.ghostPath, len(g.ghostPath), colGhost)
			label(screen, g.ghostPath[len(g.ghostPath)-1], "what really would have happened")
		}
		// the plain model's guess and the trick's answer for the other heading
		ring(screen, g.guessInstead, colGuess)
		label(screen, g.guessInstead, "plain model")
		dot(screen, g.answer, 7, colAnswer)
		label(screen, g.answer, "what-if answer")
		// the copied current: guess -> answer
		gx, gy := toScreen(g.guessInstead)
		ax, ay := toScreen(g.answer)
		vector.StrokeLine(screen, gx, gy, ax, ay, 1, colAnswer, true)
	}

	g.drawHUD(screen)
}

func (g *Game) drawHUD(screen *ebiten.Image) {
	var step string
	switch g.phase {
	case aiming:
		step = "1. Aim with the mouse and click to sail. There's a current you can't see."
	case sailing:
		step = "   Sailing..."
	case whatIf:
		step = "2. Now aim a DIFFERENT heading. Blue = what-if answer. Click to replay with the same current."
	case replaying:
		step = "3. Replaying your trip with the same current..."
	case scored:
		winner := "the what-if trick"
		if g.lastPlain < g.lastTrick {
			winner = "the plain model (it happens!)"
		}
		step = fmt.Sprintf("3. Miss: plain model %.2f boat lengths, what-if trick %.2f. Closer: %s. Click for a new trip.",
			g.lastPlain, g.lastTrick, winner)
	}
	ebitenutil.DebugPrintAt(screen, step, 12, 10)

	cur := "C: show current"
	if g.showCurrent {
		cur = "C: hide current"
	}
	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("Trip length: %.2fs (Up/Down while aiming)   %s   R: new trip", g.h, cur), 12, screenH-44)

	if g.trips > 0 {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("After %d trips, average miss: plain model %.2f, what-if trick %.2f",
			g.trips, g.plainMiss/float64(g.trips), g.trickMiss/float64(g.trips)), 12, screenH-24)
	}
}

func (g *Game) Layout(int, int) (int, int) { return screenW, screenH }

func main() {
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("What-If Boat")
	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}
