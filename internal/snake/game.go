// internal/snake/game.go — full replacement
// Fixed: food collision is checked against exact head position after move

package snake

import (
	"math/rand"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

type Point struct{ X, Y int }

type PlayerStatus uint8

const (
	StatusAlive PlayerStatus = iota
	StatusDead
	StatusWon
)

type Player struct {
	ID      uint8
	Name    string
	Body    []Point
	Dir     protocol.Direction
	NextDir protocol.Direction
	Score   uint16
	Status  PlayerStatus
	Level   uint8
}

type LevelConfig struct {
	FoodCount   int
	PoisonCount int
}

var Levels = []LevelConfig{
	{FoodCount: 1, PoisonCount: 0},
	{FoodCount: 2, PoisonCount: 1},
	{FoodCount: 2, PoisonCount: 2},
	{FoodCount: 3, PoisonCount: 2},
	{FoodCount: 3, PoisonCount: 3},
	{FoodCount: 4, PoisonCount: 3},
	{FoodCount: 4, PoisonCount: 4},
	{FoodCount: 5, PoisonCount: 4},
	{FoodCount: 5, PoisonCount: 5},
	{FoodCount: 6, PoisonCount: 5},
}

const (
	MaxLevel       = 10
	FoodToNextLevel = 5
)

type Game struct {
	Width     int
	Height    int
	Players   []*Player
	Foods     []Point
	Poisons   []Point
	Level     uint8
	TickCount uint64
	rng       *rand.Rand
}

func NewGame(width, height int, rng *rand.Rand) *Game {
	g := &Game{
		Width:  width,
		Height: height,
		Level:  1,
		rng:    rng,
	}
	g.spawnItems()
	return g
}

func (g *Game) levelConfig() LevelConfig {
	idx := int(g.Level) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(Levels) {
		idx = len(Levels) - 1
	}
	return Levels[idx]
}

func (g *Game) spawnItems() {
	cfg := g.levelConfig()
	g.Foods   = make([]Point, 0, cfg.FoodCount)
	g.Poisons = make([]Point, 0, cfg.PoisonCount)
	for i := 0; i < cfg.FoodCount; i++ {
		g.Foods = append(g.Foods, g.randomEmptyCell())
	}
	for i := 0; i < cfg.PoisonCount; i++ {
		g.Poisons = append(g.Poisons, g.randomEmptyCell())
	}
}

func (g *Game) AddPlayer(id uint8, name string) bool {
	if len(g.Players) >= protocol.MaxPlayers {
		return false
	}

	qx  := g.Width / 4
	q3x := (g.Width * 3) / 4
	qy  := g.Height / 2
	qt  := g.Height / 4
	qb  := (g.Height * 3) / 4

	starts := []struct {
		head Point
		dir  protocol.Direction
	}{
		{Point{qx, qy}, protocol.DirRight},
		{Point{q3x, qy}, protocol.DirLeft},
		{Point{g.Width / 2, qt}, protocol.DirDown},
		{Point{g.Width / 2, qb}, protocol.DirUp},
	}

	start := starts[len(g.Players)%4]
	body  := make([]Point, 3)
	body[0] = start.head
	for i := 1; i < 3; i++ {
		body[i] = stepBack(start.head, start.dir, i)
	}

	g.Players = append(g.Players, &Player{
		ID:      id,
		Name:    name,
		Body:    body,
		Dir:     start.dir,
		NextDir: start.dir,
		Status:  StatusAlive,
		Level:   g.Level,
	})
	return true
}

func (g *Game) RemovePlayer(id uint8) {
	for _, p := range g.Players {
		if p.ID == id {
			p.Status = StatusDead
		}
	}
}

func (g *Game) SetDirection(playerID uint8, dir protocol.Direction) {
	for _, p := range g.Players {
		if p.ID == playerID && p.Status == StatusAlive {
			if !isOpposite(p.Dir, dir) {
				p.NextDir = dir
			}
		}
	}
}

func (g *Game) LevelUp() bool {
	if g.Level >= MaxLevel {
		return false
	}
	g.Level++

	qx  := g.Width / 4
	q3x := (g.Width * 3) / 4
	qy  := g.Height / 2
	qt  := g.Height / 4
	qb  := (g.Height * 3) / 4

	starts := []struct {
		head Point
		dir  protocol.Direction
	}{
		{Point{qx, qy}, protocol.DirRight},
		{Point{q3x, qy}, protocol.DirLeft},
		{Point{g.Width / 2, qt}, protocol.DirDown},
		{Point{g.Width / 2, qb}, protocol.DirUp},
	}

	for i, p := range g.Players {
		if p.Status == StatusAlive {
			start := starts[i%4]
			body  := make([]Point, 3)
			body[0] = start.head
			for j := 1; j < 3; j++ {
				body[j] = stepBack(start.head, start.dir, j)
			}
			p.Body    = body
			p.Dir     = start.dir
			p.NextDir = start.dir
			p.Level   = g.Level
		}
	}
	g.spawnItems()
	return true
}

type TickResult uint8

const (
	TickContinue     TickResult = iota
	TickAllDead
	TickLevelComplete
	TickGameComplete
)

func (g *Game) Tick() TickResult {
	g.TickCount++

	// Step 1: apply buffered direction changes
	for _, p := range g.Players {
		if p.Status == StatusAlive {
			p.Dir = p.NextDir
		}
	}

	// Step 2: compute where each head will move
	newHeads := make(map[uint8]Point)
	for _, p := range g.Players {
		if p.Status == StatusAlive {
			newHeads[p.ID] = step(p.Body[0], p.Dir)
		}
	}

	// Step 3: collision detection against walls, self, others, poison
	for _, p := range g.Players {
		if p.Status != StatusAlive {
			continue
		}
		head := newHeads[p.ID]

		// Wall
		if head.X < 0 || head.X >= g.Width ||
			head.Y < 0 || head.Y >= g.Height {
			p.Status = StatusDead
			continue
		}

		// Self — check all body segments except the last
		// (tail will move away this tick, so exclude it)
		checkBody := p.Body
		if len(checkBody) > 1 {
			checkBody = checkBody[:len(checkBody)-1]
		}
		hitSelf := false
		for _, seg := range checkBody {
			if head == seg {
				hitSelf = true
				break
			}
		}
		if hitSelf {
			p.Status = StatusDead
			continue
		}

		// Other snakes — head into any body segment
		hitOther := false
		for _, other := range g.Players {
			if other.ID == p.ID || other.Status != StatusAlive {
				continue
			}
			for _, seg := range other.Body {
				if head == seg {
					hitOther = true
					break
				}
			}
			if hitOther {
				break
			}
			// Head-on collision
			if oh, ok := newHeads[other.ID]; ok && oh == head {
				hitOther = true
				break
			}
		}
		if hitOther {
			p.Status = StatusDead
			continue
		}

		// Poison
		for _, poison := range g.Poisons {
			if head == poison {
				p.Status = StatusDead
				break
			}
		}
	}

	// Step 4: move snakes and check food.
	// CRITICAL: food check uses the SAME newHead used for collision.
	// This guarantees: if the head reaches food position, food is eaten.
	// The check is exact point equality — no floating point, no off-by-one.
	for _, p := range g.Players {
		if p.Status != StatusAlive {
			continue
		}
		head := newHeads[p.ID]

		// Check if head lands on any food
		atFood := -1
		for i, food := range g.Foods {
			if head.X == food.X && head.Y == food.Y {
				atFood = i
				break
			}
		}

		if atFood >= 0 {
			// Grow: prepend new head, keep tail
			p.Body = append([]Point{head}, p.Body...)
			p.Score++
			// Replace eaten food immediately with a new one
			// Place it away from all current positions
			g.Foods[atFood] = g.randomEmptyCell()
		} else {
			// Normal move: prepend new head, drop tail
			newBody := make([]Point, len(p.Body))
			newBody[0] = head
			copy(newBody[1:], p.Body[:len(p.Body)-1])
			p.Body = newBody
		}
	}

	// Step 5: count alive players
	alive := 0
	for _, p := range g.Players {
		if p.Status == StatusAlive {
			alive++
		}
	}

	if alive == 0 {
		return TickAllDead
	}

	// Multiplayer last-standing: one remains
	if len(g.Players) > 1 && alive == 1 {
		return TickAllDead
	}

	// Step 6: level completion — cumulative score threshold
	totalScore := uint16(0)
	for _, p := range g.Players {
		if p.Status == StatusAlive {
			totalScore += p.Score
		}
	}
	threshold := uint16(int(g.Level) * FoodToNextLevel)
	if totalScore >= threshold {
		if g.Level >= MaxLevel {
			return TickGameComplete
		}
		return TickLevelComplete
	}

	return TickContinue
}

func (g *Game) Board() []protocol.Cell {
	board := make([]protocol.Cell, g.Width*g.Height)

	for _, food := range g.Foods {
		if food.X >= 0 && food.X < g.Width &&
			food.Y >= 0 && food.Y < g.Height {
			board[food.Y*g.Width+food.X] = protocol.CellFood
		}
	}

	for _, poison := range g.Poisons {
		if poison.X >= 0 && poison.X < g.Width &&
			poison.Y >= 0 && poison.Y < g.Height {
			board[poison.Y*g.Width+poison.X] = protocol.CellPoison
		}
	}

	for _, p := range g.Players {
		if p.Status != StatusAlive {
			continue
		}
		bodyCell := protocol.Cell(uint8(protocol.CellSnake1) + p.ID)
headCell := protocol.Cell(uint8(protocol.CellHead1)  + p.ID)
		for i, seg := range p.Body {
			idx := seg.Y*g.Width + seg.X
			if idx >= 0 && idx < len(board) {
				if i == 0 {
					board[idx] = headCell
				} else {
					board[idx] = bodyCell
				}
			}
		}
	}
	return board
}

func (g *Game) Scores() []uint16 {
	scores := make([]uint16, len(g.Players))
	for i, p := range g.Players {
		scores[i] = p.Score
	}
	return scores
}

func (g *Game) Winner() uint8 {
	var alive []*Player
	for _, p := range g.Players {
		if p.Status == StatusAlive {
			alive = append(alive, p)
		}
	}

	if len(alive) == 1 {
		return alive[0].ID
	}
	if len(alive) > 1 {
		var best *Player
		for _, p := range alive {
			if best == nil || p.Score > best.Score {
				best = p
			}
		}
		return best.ID
	}

	// All dead
	if len(g.Players) == 1 {
		return 0xFF // single player loss
	}

	var best *Player
	for _, p := range g.Players {
		if best == nil || p.Score > best.Score {
			best = p
		}
	}
	if best != nil {
		return best.ID
	}
	return 0xFF
}

// ─────────────────────────────────────────────────────────────
// HELPERS
// ─────────────────────────────────────────────────────────────

func step(p Point, d protocol.Direction) Point {
	switch d {
	case protocol.DirUp:    return Point{p.X, p.Y - 1}
	case protocol.DirDown:  return Point{p.X, p.Y + 1}
	case protocol.DirLeft:  return Point{p.X - 1, p.Y}
	case protocol.DirRight: return Point{p.X + 1, p.Y}
	}
	return p
}

func stepBack(p Point, d protocol.Direction, n int) Point {
	for i := 0; i < n; i++ {
		switch d {
		case protocol.DirUp:    p.Y++
		case protocol.DirDown:  p.Y--
		case protocol.DirLeft:  p.X++
		case protocol.DirRight: p.X--
		}
	}
	return p
}

func isOpposite(a, b protocol.Direction) bool {
	return (a == protocol.DirUp    && b == protocol.DirDown)  ||
		   (a == protocol.DirDown  && b == protocol.DirUp)    ||
		   (a == protocol.DirLeft  && b == protocol.DirRight) ||
		   (a == protocol.DirRight && b == protocol.DirLeft)
}

func (g *Game) randomEmptyCell() Point {
	occupied := make(map[Point]bool)
	for _, p := range g.Players {
		for _, seg := range p.Body {
			occupied[seg] = true
		}
	}
	for _, f := range g.Foods {
		occupied[f] = true
	}
	for _, poison := range g.Poisons {
		occupied[poison] = true
	}

	var empty []Point
	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			pt := Point{x, y}
			if !occupied[pt] {
				empty = append(empty, pt)
			}
		}
	}
	if len(empty) == 0 {
		return Point{g.Width / 2, g.Height / 2}
	}
	return empty[g.rng.Intn(len(empty))]
}