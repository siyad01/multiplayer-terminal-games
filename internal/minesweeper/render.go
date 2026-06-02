// internal/minesweeper/render.go
// Single responsibility: draw the Minesweeper board to a tcell screen.
// Knows about terminals. Knows nothing about game logic.

package minesweeper

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

// ─────────────────────────────────────────────────────────────
// STYLES
// ─────────────────────────────────────────────────────────────

var (
	styleDefault  = tcell.StyleDefault
	styleHidden   = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleFlagged  = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	styleMine     = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleExploded = tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorWhite).Bold(true)
	styleEmpty    = tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
	styleUI       = tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	styleUILabel  = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleWin      = tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
	styleDead     = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleCursor   = tcell.StyleDefault.Background(tcell.ColorNavy)

	// Number colors — classic Minesweeper colors
	numberStyles = []tcell.Style{
		tcell.StyleDefault.Foreground(tcell.ColorGray),        // 0 (empty)
		tcell.StyleDefault.Foreground(tcell.ColorBlue).Bold(true),   // 1
		tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true),  // 2
		tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true),    // 3
		tcell.StyleDefault.Foreground(tcell.ColorNavy).Bold(true),   // 4
		tcell.StyleDefault.Foreground(tcell.ColorMaroon).Bold(true), // 5
		tcell.StyleDefault.Foreground(tcell.ColorTeal).Bold(true),   // 6
		tcell.StyleDefault.Foreground(tcell.ColorPurple).Bold(true), // 7
		tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Bold(true), // 8
	}

	// Player cursor colors
	cursorColors = []tcell.Color{
		tcell.ColorGreen,
		tcell.ColorBlue,
		tcell.ColorYellow,
		tcell.ColorFuchsia,
	}
)

// ─────────────────────────────────────────────────────────────
// RENDERER
// ─────────────────────────────────────────────────────────────

type Renderer struct {
	screen    tcell.Screen
	playerID  uint8
	names     map[uint8]string
	gameMode  Mode
	width     uint8
	height    uint8

	// Local cursor position (this client)
	CursorX uint8
	CursorY uint8

	// All player cursors (from server)
	cursors []protocol.CursorPos
}

func NewRenderer(screen tcell.Screen, playerID uint8, name string) *Renderer {
	return &Renderer{
		screen:   screen,
		playerID: playerID,
		names:    map[uint8]string{playerID: name},
		CursorX:  0,
		CursorY:  0,
	}
}

func (r *Renderer) SetMode(m Mode)           { r.gameMode = m }
func (r *Renderer) AddPlayer(id uint8, name string) { r.names[id] = name }
func (r *Renderer) SetCursors(c []protocol.CursorPos) { r.cursors = c }

// ─────────────────────────────────────────────────────────────
// DRAW BOARD — main render function
// ─────────────────────────────────────────────────────────────

func (r *Renderer) DrawBoard(
	width, height uint8,
	cells []protocol.MineCell,
	scores []uint16,
	flagsLeft uint16,
	playerNames map[uint8]string,
) {
	r.screen.Clear()
	r.width  = width
	r.height = height

	termW, termH := r.screen.Size()

	// Each cell = 2 chars wide, 1 char tall (for readability)
	boardW := int(width)*2 + 4   // 2 border + 2 padding
	boardH := int(height) + 2    // top/bottom border
	panelW := 22
	gap    := 2

	totalW  := boardW + gap + panelW
	offsetX := (termW - totalW) / 2
	if offsetX < 0 { offsetX = 0 }
	offsetY := (termH - boardH) / 2
	if offsetY < 3 { offsetY = 3 }

	panelX := offsetX + boardW + gap
	panelY := offsetY

	// ── Header ────────────────────────────────────────────────
	modeName := r.modeStr()
	r.put(offsetX+(boardW-len(modeName))/2, offsetY-2,
		modeName,
		tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true))

	mineStr := fmt.Sprintf("mines: %d  flags left: %d",
		0, flagsLeft) // mine count unknown to client
	r.put(offsetX, offsetY-1, mineStr, styleUILabel)

	// ── Board border ──────────────────────────────────────────
	r.drawBorder(offsetX, offsetY, int(width)*2+2, int(height))

	// ── Cells ─────────────────────────────────────────────────
	// Build cursor lookup for fast access
	cursorAt := make(map[[2]uint8]uint8) // [x,y] → playerID
	for _, c := range r.cursors {
		cursorAt[[2]uint8{c.X, c.Y}] = c.PlayerID
	}
	// Always show own cursor
	cursorAt[[2]uint8{r.CursorX, r.CursorY}] = r.playerID

	for cy := 0; cy < int(height); cy++ {
		for cx := 0; cx < int(width); cx++ {
			cell    := cells[cy*int(width)+cx]
			screenX := offsetX + 2 + cx*2
			screenY := offsetY + 1 + cy

			// Check if any cursor is here
			pid, hasCursor := cursorAt[[2]uint8{uint8(cx), uint8(cy)}]
			isOwnCursor    := hasCursor && pid == r.playerID

			r.drawCell(screenX, screenY, cell, hasCursor, isOwnCursor, pid)
		}
	}

	// ── Side panel ────────────────────────────────────────────
	r.drawPanel(panelX, panelY, scores, playerNames, flagsLeft)

	// ── Controls ──────────────────────────────────────────────
	r.put(offsetX, offsetY+boardH+1,
		"arrows move  ENTER reveal  F flag  Q quit", styleUILabel)

	r.screen.Show()
}

func (r *Renderer) modeStr() string {
	switch r.gameMode {
	case ModeSolo:        return "MINESWEEPER  -  SOLO"
	case ModeCooperative: return "MINESWEEPER  -  COOPERATIVE"
	case ModeCompetitive: return "MINESWEEPER  -  COMPETITIVE"
	}
	return "MINESWEEPER"
}

// ─────────────────────────────────────────────────────────────
// CELL RENDERING
// ─────────────────────────────────────────────────────────────

func (r *Renderer) drawCell(
	tx, ty int,
	cell protocol.MineCell,
	hasCursor, isOwn bool,
	cursorPlayerID uint8,
) {
	ch1, ch2, style := r.cellChars(cell)

	// Override background for cursors
	if hasCursor {
		col := cursorColors[cursorPlayerID%uint8(len(cursorColors))]
		if isOwn {
			// Own cursor: bright background
			style = style.Background(tcell.ColorNavy)
		} else {
			// Other player's cursor: colored underline effect
			style = style.Background(col).Foreground(tcell.ColorBlack)
		}
	}

	r.screen.SetContent(tx,   ty, ch1, nil, style)
	r.screen.SetContent(tx+1, ty, ch2, nil, style)
}

func (r *Renderer) cellChars(cell protocol.MineCell) (rune, rune, tcell.Style) {
	state := cell & 0xF0
	value := cell & 0x0F

	switch state {
	case 0x00: // hidden
		return '[', ']', styleHidden

	case 0x10: // flagged
		return '[', 'F', styleFlagged

	case 0x20: // exploded
		return '[', 'X', styleExploded

	case 0x30: // revealed
		if value == 9 {
			// Mine revealed (game over)
			return '[', '*', styleMine
		}
		if value == 0 {
			return ' ', ' ', styleEmpty
		}
		n := int(value)
		if n < len(numberStyles) {
			return ' ', rune('0'+value), numberStyles[n]
		}
		return ' ', rune('0'+value), styleDefault
	}

	return '[', ']', styleHidden
}

// ─────────────────────────────────────────────────────────────
// SIDE PANEL
// ─────────────────────────────────────────────────────────────

func (r *Renderer) drawPanel(
	x, y int,
	scores []uint16,
	names map[uint8]string,
	flagsLeft uint16,
) {
	r.put(x, y,   "+--------------------+", styleUILabel)
	r.put(x, y+1, "|    SCOREBOARD      |", styleUI)
	r.put(x, y+2, "+--------------------+", styleUILabel)

	for i, score := range scores {
		id   := uint8(i)
		name := names[id]
		if name == "" { name = fmt.Sprintf("P%d", id+1) }
		if len(name) > 9 { name = name[:8] + "." }

		col   := cursorColors[i%len(cursorColors)]
		style := tcell.StyleDefault.Foreground(col).Bold(true)
		mark  := " "
		if id == r.playerID { mark = ">" }

		line := fmt.Sprintf("|%s %-9s %7d |", mark, name, score)
		r.put(x, y+3+i, line, style)
	}
	for i := len(scores); i < 4; i++ {
		r.put(x, y+3+i, "|                    |", styleUILabel)
	}
	r.put(x, y+7, "+--------------------+", styleUILabel)

	// Legend
	r.put(x, y+9,  "+--------------------+", styleUILabel)
	r.put(x, y+10, "|       LEGEND       |", styleUI)
	r.put(x, y+11, "+--------------------+", styleUILabel)
	r.put(x, y+12, "| [] hidden cell     |", styleHidden)
	r.put(x, y+13, "| [F] flagged mine   |", styleFlagged)
	r.put(x, y+14, "| [*] mine           |", styleMine)
	r.put(x, y+15, "| [X] exploded!      |", styleExploded)
	r.put(x, y+16, "|  1  safe number    |", numberStyles[1])
	r.put(x, y+17, "+--------------------+", styleUILabel)

	// Flags counter
	flagStr := fmt.Sprintf("| flags left: %-6d |", flagsLeft)
	r.put(x, y+18, flagStr, styleFlagged)
	r.put(x, y+19, "+--------------------+", styleUILabel)
}

// ─────────────────────────────────────────────────────────────
// GAME OVER / WIN SCREENS
// ─────────────────────────────────────────────────────────────

func (r *Renderer) DrawWin(scores []uint16, names map[uint8]string) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	cx := func(s string) int {
		x := (termW - len(s)) / 2
		if x < 0 { return 0 }
		return x
	}

	y := termH/2 - 4
	r.put(cx("BOARD CLEARED!"), y, "BOARD CLEARED!", styleWin)

	if len(scores) > 0 {
		// Find winner
		var bestID uint8
		var best   uint16
		for i, s := range scores {
			if s > best {
				best   = s
				bestID = uint8(i)
			}
		}
		name := names[bestID]
		if name == "" { name = fmt.Sprintf("Player %d", bestID) }

		if r.gameMode == ModeSolo {
			r.put(cx("You cleared the board!"), y+2,
				"You cleared the board!", styleWin)
		} else {
			msg := fmt.Sprintf("Most reveals: %s (%d cells)", name, best)
			r.put(cx(msg), y+2, msg, styleWin)
		}
	}

	r.put(cx("R  ->  play again"), y+5, "R  ->  play again", styleWin)
	r.put(cx("Q  ->  quit"),       y+7, "Q  ->  quit",       styleUILabel)
	r.screen.Show()
}

func (r *Renderer) DrawLose(
	hitPlayerID uint8,
	hitX, hitY uint8,
	names map[uint8]string,
) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	cx := func(s string) int {
		x := (termW - len(s)) / 2
		if x < 0 { return 0 }
		return x
	}

	y := termH/2 - 4

	if hitPlayerID == r.playerID {
		r.put(cx("YOU HIT A MINE!"), y, "YOU HIT A MINE!", styleDead)
		sub := fmt.Sprintf("at cell (%d, %d)", hitX, hitY)
		r.put(cx(sub), y+2, sub, styleUILabel)
	} else {
		name := names[hitPlayerID]
		if name == "" { name = fmt.Sprintf("Player %d", hitPlayerID) }
		msg := fmt.Sprintf("%s hit a mine at (%d,%d)!", name, hitX, hitY)
		r.put(cx(msg), y, msg, styleDead)
	}

	r.put(cx("R  ->  play again"), y+5, "R  ->  play again", styleWin)
	r.put(cx("Q  ->  quit"),       y+7, "Q  ->  quit",       styleUILabel)
	r.screen.Show()
}

func (r *Renderer) DrawLobby(
	myName  string,
	players []string,
	isHost  bool,
	mode    Mode,
	diff    string,
) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	type row struct {
		text  string
		style tcell.Style
	}

	modeNames := map[Mode]string{
		ModeSolo:        "MINESWEEPER  -  SOLO",
		ModeCooperative: "MINESWEEPER  -  COOPERATIVE",
		ModeCompetitive: "MINESWEEPER  -  COMPETITIVE",
	}

	rows := []row{
		{"", styleDefault},
		{modeNames[mode], styleUI},
		{"", styleDefault},
		{"-- players --", styleUILabel},
		{"", styleDefault},
	}

	for i, p := range players {
		prefix := "   "
		if i == 0 { prefix = "[H] " }
		col := cursorColors[i%len(cursorColors)]
		rows = append(rows, row{
			prefix + p,
			tcell.StyleDefault.Foreground(col).Bold(true),
		})
	}

	rows = append(rows, row{"", styleDefault})

	diffNames := map[string]string{
		"easy":   "Easy   ( 9x9,  10 mines)",
		"medium": "Medium (16x16, 40 mines)",
		"hard":   "Hard   (30x16, 99 mines)",
	}

	if isHost {
		rows = append(rows,
			row{fmt.Sprintf("Difficulty: %s", diffNames[diff]),
				tcell.StyleDefault.Foreground(tcell.ColorYellow)},
			row{"D  -  cycle difficulty", styleUILabel},
		)

		if mode != ModeSolo {
			rows = append(rows,
				row{"M  -  cycle mode (Cooperative / Competitive)", styleUILabel},
			)
		}

		rows = append(rows,
			row{"", styleDefault},
			row{"SPACE or ENTER  -  start", styleWin},
		)
	} else {
		rows = append(rows,
			row{fmt.Sprintf("Difficulty: %s", diffNames[diff]), styleUILabel},
			row{"waiting for host to start...", styleUILabel},
		)
	}

	rows = append(rows,
		row{"", styleDefault},
		row{"Q to quit", styleUILabel},
	)

	startY := termH/2 - len(rows)/2
	if startY < 0 { startY = 0 }
	for i, r2 := range rows {
		x := (termW - len(r2.text)) / 2
		if x < 0 { x = 0 }
		r.put(x, startY+i, r2.text, r2.style)
	}
	r.screen.Show()
}

// ─────────────────────────────────────────────────────────────
// PRIMITIVES
// ─────────────────────────────────────────────────────────────

func (r *Renderer) drawBorder(ox, oy, w, h int) {
	s := styleUILabel
	for x := 0; x <= w+1; x++ {
		r.screen.SetContent(ox+x, oy,   '-', nil, s)
		r.screen.SetContent(ox+x, oy+h+1, '-', nil, s)
	}
	for y := 0; y <= h+1; y++ {
		r.screen.SetContent(ox,     oy+y, '|', nil, s)
		r.screen.SetContent(ox+w+1, oy+y, '|', nil, s)
	}
	r.screen.SetContent(ox,     oy,     '+', nil, s)
	r.screen.SetContent(ox+w+1, oy,     '+', nil, s)
	r.screen.SetContent(ox,     oy+h+1, '+', nil, s)
	r.screen.SetContent(ox+w+1, oy+h+1, '+', nil, s)
}

func (r *Renderer) put(x, y int, s string, style tcell.Style) {
	for i, ch := range s {
		r.screen.SetContent(x+i, y, ch, nil, style)
	}
}