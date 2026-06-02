// internal/minesweeper/game.go
// Single responsibility: Minesweeper board state and rules.
// Pure logic — no networking, no goroutines.

package minesweeper

import (
	"math/rand"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

// ─────────────────────────────────────────────────────────────
// TYPES
// ─────────────────────────────────────────────────────────────

type CellState uint8

const (
	StateHidden   CellState = iota
	StateRevealed
	StateFlagged
	StateExploded // mine that was triggered
)

// InternalCell is the server-side cell — knows everything.
// The client only sees what's been revealed.
type InternalCell struct {
	IsMine    bool
	Adjacent  uint8 // count of adjacent mines (0-8)
	State     CellState
	FlaggedBy uint8 // player ID who flagged it
}

// Difficulty presets
type Difficulty struct {
	Width, Height uint8
	Mines         int
}

var Difficulties = map[string]Difficulty{
	"easy":   {Width: 9,  Height: 9,  Mines: 10},
	"medium": {Width: 16, Height: 16, Mines: 40},
	"hard":   {Width: 30, Height: 16, Mines: 99},
}

// GameMode for minesweeper
type Mode uint8

const (
	ModeSolo        Mode = iota // single player classic
	ModeCooperative             // shared board, work together
	ModeCompetitive             // shared board, compete for reveals
)

type Game struct {
	Width     uint8
	Height    uint8
	Cells     []InternalCell // row-major: cells[y*Width + x]
	Mode      Mode
	MineCount int
	FlagsLeft int
	Started   bool // false until first reveal (safe first click)
	Over      bool
	Won       bool
	rng       *rand.Rand

	// Per-player state
	Players  []*MinePlayer
	Cursors  map[uint8]protocol.CursorPos // playerID → cursor
}

type MinePlayer struct {
	ID      uint8
	Name    string
	Reveals uint16 // cells this player revealed
	Flags   uint16 // flags this player placed
}

// ─────────────────────────────────────────────────────────────
// CONSTRUCTOR
// ─────────────────────────────────────────────────────────────

func NewGame(d Difficulty, mode Mode, rng *rand.Rand) *Game {
	size := int(d.Width) * int(d.Height)
	g := &Game{
		Width:     d.Width,
		Height:    d.Height,
		Cells:     make([]InternalCell, size),
		Mode:      mode,
		MineCount: d.Mines,
		FlagsLeft: d.Mines,
		rng:       rng,
		Cursors:   make(map[uint8]protocol.CursorPos),
	}
	return g
	// Note: mines not placed yet — safe first click
}

func (g *Game) AddPlayer(id uint8, name string) {
	g.Players = append(g.Players, &MinePlayer{
		ID: id, Name: name,
	})
	g.Cursors[id] = protocol.CursorPos{
		PlayerID: id,
		X:        g.Width / 2,
		Y:        g.Height / 2,
	}
}

func (g *Game) RemovePlayer(id uint8) {
	for i, p := range g.Players {
		if p.ID == id {
			g.Players = append(g.Players[:i], g.Players[i+1:]...)
			break
		}
	}
	delete(g.Cursors, id)
}

// ─────────────────────────────────────────────────────────────
// MINE PLACEMENT — deferred until first click
// Guarantees first click is always safe (never a mine).
// ─────────────────────────────────────────────────────────────

func (g *Game) placeMines(safeX, safeY uint8) {
	w, h := int(g.Width), int(g.Height)

	// Safe zone: 3×3 around first click
	safe := make(map[int]bool)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := int(safeX)+dx, int(safeY)+dy
			if nx >= 0 && nx < w && ny >= 0 && ny < h {
				safe[ny*w+nx] = true
			}
		}
	}

	// Collect eligible cells
	var eligible []int
	for i := range g.Cells {
		if !safe[i] {
			eligible = append(eligible, i)
		}
	}

	// Fisher-Yates shuffle, take first MineCount
	for i := len(eligible) - 1; i > 0; i-- {
		j := g.rng.Intn(i + 1)
		eligible[i], eligible[j] = eligible[j], eligible[i]
	}

	mineCount := g.MineCount
	if mineCount > len(eligible) {
		mineCount = len(eligible)
	}
	for _, idx := range eligible[:mineCount] {
		g.Cells[idx].IsMine = true
	}

	// Calculate adjacent counts
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if g.Cells[y*w+x].IsMine {
				continue
			}
			count := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < w && ny >= 0 && ny < h {
						if g.Cells[ny*w+nx].IsMine {
							count++
						}
					}
				}
			}
			g.Cells[y*w+x].Adjacent = uint8(count)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// ACTIONS
// ─────────────────────────────────────────────────────────────

type ActionResult struct {
	Changes  []protocol.MineChange
	Exploded bool   // hit a mine
	Won      bool   // all non-mines revealed
	PlayerID uint8  // who triggered this
}

// Reveal reveals cell (x,y) for playerID.
// Returns all cells that changed state (flood-fill for empty cells).
func (g *Game) Reveal(playerID, x, y uint8) ActionResult {
	if g.Over {
		return ActionResult{}
	}

	idx := int(y)*int(g.Width) + int(x)
	cell := &g.Cells[idx]

	// Already revealed or flagged — ignore
	if cell.State == StateRevealed || cell.State == StateFlagged {
		return ActionResult{}
	}

	// First click: place mines now, guaranteeing this cell is safe
	if !g.Started {
		g.Started = true
		g.placeMines(x, y)
		// Re-read cell after mine placement
		cell = &g.Cells[idx]
	}

	var changes []protocol.MineChange

	// Hit a mine
	if cell.IsMine {
		cell.State = StateExploded
		g.Over = true
		changes = append(changes, protocol.MineChange{
			X: x, Y: y,
			Cell: protocol.MineCellExploded,
		})
		// Reveal all mines on game over
		for i := range g.Cells {
			if g.Cells[i].IsMine && g.Cells[i].State == StateHidden {
				g.Cells[i].State = StateRevealed
				cy := uint8(i / int(g.Width))
				cx := uint8(i % int(g.Width))
				changes = append(changes, protocol.MineChange{
					X: cx, Y: cy,
					Cell: protocol.MineCell(0x30 + 9), // mine revealed
				})
			}
		}
		return ActionResult{
			Changes:  changes,
			Exploded: true,
			PlayerID: playerID,
		}
	}

	// Safe cell — flood fill
	changes = g.floodReveal(playerID, x, y)

	// Update player's reveal count
	for _, p := range g.Players {
		if p.ID == playerID {
			p.Reveals += uint16(len(changes))
			break
		}
	}

	// Check win
	won := g.checkWin()
	if won {
		g.Over = true
		g.Won  = true
	}

	return ActionResult{
		Changes:  changes,
		Won:      won,
		PlayerID: playerID,
	}
}

// floodReveal reveals cell and recursively reveals neighbors if adjacent=0.
// Standard Minesweeper flood-fill.
func (g *Game) floodReveal(playerID, x, y uint8) []protocol.MineChange {
	w, h := int(g.Width), int(g.Height)
	var changes []protocol.MineChange

	// BFS queue
	type pt struct{ x, y int }
	queue   := []pt{{int(x), int(y)}}
	visited := make(map[int]bool)

	for len(queue) > 0 {
		curr  := queue[0]
		queue = queue[1:]

		idx := curr.y*w + curr.x
		if visited[idx] {
			continue
		}
		visited[idx] = true

		cell := &g.Cells[idx]
		if cell.State == StateRevealed || cell.IsMine {
			continue
		}
		if cell.State == StateFlagged {
			continue // don't auto-reveal flagged cells
		}

		cell.State = StateRevealed

		// Encode visible cell value
		var mc protocol.MineCell
		if cell.IsMine {
			mc = protocol.MineCell(0x30 + 9)
		} else {
			mc = protocol.MineCell(0x30 + cell.Adjacent)
		}
		changes = append(changes, protocol.MineChange{
			X: uint8(curr.x), Y: uint8(curr.y), Cell: mc,
		})

		// If empty (0 adjacent mines) — expand to all 8 neighbors
		if cell.Adjacent == 0 {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := curr.x+dx, curr.y+dy
					if nx >= 0 && nx < w && ny >= 0 && ny < h {
						ni := ny*w + nx
						if !visited[ni] {
							queue = append(queue, pt{nx, ny})
						}
					}
				}
			}
		}
	}

	return changes
}

// Flag toggles a flag on cell (x,y).
func (g *Game) Flag(playerID, x, y uint8) []protocol.MineChange {
	if g.Over {
		return nil
	}
	idx  := int(y)*int(g.Width) + int(x)
	cell := &g.Cells[idx]

	if cell.State == StateRevealed {
		return nil
	}

	var newState CellState
	var mc      protocol.MineCell

	switch cell.State {
	case StateHidden:
		if g.FlagsLeft <= 0 {
			return nil // no flags left
		}
		cell.State     = StateFlagged
		cell.FlaggedBy = playerID
		g.FlagsLeft--
		newState = StateFlagged
		mc       = protocol.MineCellFlagged

		for _, p := range g.Players {
			if p.ID == playerID {
				p.Flags++
				break
			}
		}

	case StateFlagged:
		cell.State     = StateHidden
		cell.FlaggedBy = 0
		g.FlagsLeft++
		newState = StateHidden
		mc       = protocol.MineCellHidden
		_ = newState

		for _, p := range g.Players {
			if p.ID == playerID && p.Flags > 0 {
				p.Flags--
				break
			}
		}
	}

	return []protocol.MineChange{{X: x, Y: y, Cell: mc}}
}

// MoveCursor updates a player's cursor position.
func (g *Game) MoveCursor(playerID, x, y uint8) {
	g.Cursors[playerID] = protocol.CursorPos{
		PlayerID: playerID, X: x, Y: y,
	}
}

// ─────────────────────────────────────────────────────────────
// STATE QUERIES
// ─────────────────────────────────────────────────────────────

func (g *Game) checkWin() bool {
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.IsMine && c.State != StateRevealed {
			return false
		}
	}
	return true
}

// VisibleCells returns cells as the client sees them.
// Hidden cells show as hidden. Mines only visible after game over.
func (g *Game) VisibleCells() []protocol.MineCell {
	cells := make([]protocol.MineCell, len(g.Cells))
	for i, c := range g.Cells {
		switch c.State {
		case StateHidden:
			cells[i] = protocol.MineCellHidden
		case StateFlagged:
			cells[i] = protocol.MineCellFlagged
		case StateExploded:
			cells[i] = protocol.MineCellExploded
		case StateRevealed:
			if c.IsMine {
				cells[i] = protocol.MineCell(0x30 + 9) // mine
			} else {
				cells[i] = protocol.MineCell(0x30 + c.Adjacent)
			}
		}
	}
	return cells
}

func (g *Game) Scores() []uint16 {
	scores := make([]uint16, len(g.Players))
	for i, p := range g.Players {
		scores[i] = p.Reveals
	}
	return scores
}

func (g *Game) AllCursors() []protocol.CursorPos {
	cursors := make([]protocol.CursorPos, 0, len(g.Cursors))
	for _, c := range g.Cursors {
		cursors = append(cursors, c)
	}
	return cursors
}

// NonMineCount returns total non-mine cells (target for win).
func (g *Game) NonMineCount() int {
	count := 0
	for _, c := range g.Cells {
		if !c.IsMine {
			count++
		}
	}
	return count
}