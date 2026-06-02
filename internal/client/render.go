// internal/client/render.go — full replacement
// ASCII-safe scoreboard, block snake, correct layout

package client

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

var (
	styleDefault = tcell.StyleDefault
	styleWall    = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleFood    = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	stylePoison  = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleUI      = tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	styleUILabel = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleDead    = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleWin     = tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)

	snakeColors = []tcell.Color{
		tcell.ColorGreen,
		tcell.ColorBlue,
		tcell.ColorRed,
		tcell.ColorFuchsia,
	}

	// Fruit per level — emoji placed cell by cell, not in strings
	fruits = []rune{
		'🍎', '🍊', '🍋', '🍇', '🍓',
		'🍑', '🍍', '🥝', '🍒', '🫐',
	}
)

type Renderer struct {
	screen   tcell.Screen
	playerID uint8
	names    map[uint8]string
	level    uint8
	gameMode protocol.GameMode
}

func NewRenderer(playerID uint8, name string) (*Renderer, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("screen: %w", err)
	}
	if err := s.Init(); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	s.SetStyle(styleDefault)
	s.Clear()
	return &Renderer{
		screen:   s,
		playerID: playerID,
		names:    map[uint8]string{playerID: name},
		level:    1,
		gameMode: protocol.ModeSinglePlayer,
	}, nil
}

func (r *Renderer) Close()                          { r.screen.Fini() }
func (r *Renderer) Screen() tcell.Screen            { return r.screen }
func (r *Renderer) UpdatePlayerID(id uint8)         { r.playerID = id }
func (r *Renderer) SetLevel(l uint8)                { r.level = l }
func (r *Renderer) SetGameMode(m protocol.GameMode) { r.gameMode = m }
func (r *Renderer) AddPlayer(id uint8, name string) { r.names[id] = name }

// ─────────────────────────────────────────────────────────────
// DRAW GAME
// ─────────────────────────────────────────────────────────────

func (r *Renderer) DrawGame(
	width, height uint8,
	board []protocol.Cell,
	scores []uint16,
	playerNames map[uint8]string,
	secondsLeft uint16,
) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	// Each board cell = 1 char wide × 1 char tall
	boardW := int(width) + 2  // left border + cells + right border
	boardH := int(height) + 2 // top border + cells + bottom border

	// Scoreboard panel: fixed 22 chars wide
	panelW  := 22
	gap     := 2
	totalW  := boardW + gap + panelW
	offsetX := (termW - totalW) / 2
	if offsetX < 0 {
		offsetX = 0
	}

	// Center vertically, leave 2 rows above for header
	offsetY := (termH-boardH)/2 + 1
	if offsetY < 3 {
		offsetY = 3
	}

	panelX := offsetX + boardW + gap
	panelY := offsetY

	// ── Header: level centered above board ────────────────────
	levelStr := fmt.Sprintf("LEVEL  %d / 10", r.level)
	lx := offsetX + boardW/2 - len(levelStr)/2
	if lx < 0 {
		lx = 0
	}
	r.putStr(lx, offsetY-2, levelStr,
		tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true))

	// Timer: only Score Race mode, above level
	if r.gameMode == protocol.ModeScoreRace {
		mins := secondsLeft / 60
		secs := secondsLeft % 60
		ts   := fmt.Sprintf("TIME  %02d:%02d", mins, secs)
		tx   := offsetX + boardW/2 - len(ts)/2
		timerSt := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
		if secondsLeft <= 30 {
			timerSt = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		}
		if secondsLeft <= 10 {
			timerSt = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
		}
		r.putStr(tx, offsetY-3, ts, timerSt)
	}

	// ── Board border ──────────────────────────────────────────
	r.drawBorder(offsetX, offsetY, int(width), int(height))

	// ── Board cells ───────────────────────────────────────────
	for cy := 0; cy < int(height); cy++ {
		for cx := 0; cx < int(width); cx++ {
			cell := board[cy*int(width)+cx]
			r.drawCell(offsetX+1+cx, offsetY+1+cy, cell)
		}
	}

	// ── Scoreboard ────────────────────────────────────────────
	r.drawScoreboard(panelX, panelY, scores, playerNames)

	// ── Legend ────────────────────────────────────────────────
	legendY := panelY + 9
	r.drawLegend(panelX, legendY)

	// ── Mode label ────────────────────────────────────────────
	modeStr := r.modeName()
	r.putStr(panelX, panelY+20, modeStr, styleUILabel)

	// ── Controls ──────────────────────────────────────────────
	r.putStr(offsetX, offsetY+boardH+1,
		"arrows / WASD to move   Q to quit", styleUILabel)

	r.screen.Show()
}

func (r *Renderer) modeName() string {
	switch r.gameMode {
	case protocol.ModeSinglePlayer:
		return "mode: single player"
	case protocol.ModeLastStanding:
		return "mode: last standing"
	case protocol.ModeScoreRace:
		return "mode: score race   "
	}
	return ""
}

// drawCell places one board cell character at terminal position (tx,ty).
func (r *Renderer) drawCell(tx, ty int, cell protocol.Cell) {
	ch, style := r.cellAppearance(cell)
	if ch == 0 {
		return // skip — emoji placed separately
	}
	r.screen.SetContent(tx, ty, ch, nil, style)
}

func (r *Renderer) cellAppearance(cell protocol.Cell) (rune, tcell.Style) {
	switch cell {
	case protocol.CellEmpty:
		return ' ', styleDefault

	case protocol.CellFood:
		// Fruit emoji — placed as rune directly
		fruit := fruits[(int(r.level)-1)%len(fruits)]
		return fruit, styleFood

	case protocol.CellPoison:
		return '*', stylePoison // ASCII — no width issues

	case protocol.CellWall:
		return '#', styleWall

	// Snake body — solid block, identical width in all directions
	case protocol.CellSnake1:
		return '[', tcell.StyleDefault.Foreground(snakeColors[0])
	case protocol.CellSnake2:
		return '[', tcell.StyleDefault.Foreground(snakeColors[1])
	case protocol.CellSnake3:
		return '[', tcell.StyleDefault.Foreground(snakeColors[2])
	case protocol.CellSnake4:
		return '[', tcell.StyleDefault.Foreground(snakeColors[3])

	// Snake head — brighter, distinct
	case protocol.CellHead1:
		return 'O', tcell.StyleDefault.Foreground(snakeColors[0]).Bold(true)
	case protocol.CellHead2:
		return 'O', tcell.StyleDefault.Foreground(snakeColors[1]).Bold(true)
	case protocol.CellHead3:
		return 'O', tcell.StyleDefault.Foreground(snakeColors[2]).Bold(true)
	case protocol.CellHead4:
		return 'O', tcell.StyleDefault.Foreground(snakeColors[3]).Bold(true)
	}
	return ' ', styleDefault
}

// ─────────────────────────────────────────────────────────────
// BORDER — pure ASCII box drawing (safe with any font)
// ─────────────────────────────────────────────────────────────

func (r *Renderer) drawBorder(ox, oy, w, h int) {
	s := tcell.StyleDefault.Foreground(tcell.ColorGray)
	// Top and bottom edges
	for x := 0; x <= w+1; x++ {
		r.screen.SetContent(ox+x, oy, '-', nil, s)
		r.screen.SetContent(ox+x, oy+h+1, '-', nil, s)
	}
	// Left and right edges
	for y := 0; y <= h+1; y++ {
		r.screen.SetContent(ox, oy+y, '|', nil, s)
		r.screen.SetContent(ox+w+1, oy+y, '|', nil, s)
	}
	// Corners
	r.screen.SetContent(ox, oy, '+', nil, s)
	r.screen.SetContent(ox+w+1, oy, '+', nil, s)
	r.screen.SetContent(ox, oy+h+1, '+', nil, s)
	r.screen.SetContent(ox+w+1, oy+h+1, '+', nil, s)
}

// ─────────────────────────────────────────────────────────────
// SCOREBOARD — pure ASCII, no emoji in strings
// ─────────────────────────────────────────────────────────────

func (r *Renderer) drawScoreboard(
	x, y int,
	scores []uint16,
	names map[uint8]string,
) {
	// Box: 22 chars wide including borders
	r.putStr(x, y,   "+--------------------+", styleUILabel)
	r.putStr(x, y+1, "|    SCOREBOARD      |", styleUI)
	r.putStr(x, y+2, "+--------------------+", styleUILabel)

	for i, score := range scores {
		id   := uint8(i)
		name := names[id]
		if name == "" {
			name = fmt.Sprintf("P%d", id+1)
		}
		// Truncate name to fit — 9 chars max
		if len(name) > 9 {
			name = name[:8] + "."
		}

		col   := snakeColors[i%len(snakeColors)]
		style := tcell.StyleDefault.Foreground(col).Bold(true)
		mark  := " "
		if id == r.playerID {
			mark = ">"
		}

		// Carefully formatted: | mark name(9) score(5) |
		// Total inner width = 20 chars
		line := fmt.Sprintf("|%s %-9s %7d |", mark, name, score)
		r.putStr(x, y+3+i, line, style)
	}

	// Empty rows for missing players
	for i := len(scores); i < 4; i++ {
		r.putStr(x, y+3+i, "|                    |", styleUILabel)
	}
	r.putStr(x, y+7, "+--------------------+", styleUILabel)
}

// ─────────────────────────────────────────────────────────────
// LEGEND — ASCII safe
// ─────────────────────────────────────────────────────────────

func (r *Renderer) drawLegend(x, y int) {
	r.putStr(x, y,   "+--------------------+", styleUILabel)
	r.putStr(x, y+1, "|       LEGEND       |", styleUI)
	r.putStr(x, y+2, "+--------------------+", styleUILabel)

	// Food row — place emoji by cell, label by putStr
	r.putStr(x, y+3, "|  . Food (+1 score) |", styleFood)
	// Overwrite the dot with the actual fruit emoji at exact position
	fruit := fruits[(int(r.level)-1)%len(fruits)]
	r.screen.SetContent(x+3, y+3, fruit, nil, styleFood)

	// Poison row
	r.putStr(x, y+4, "|  * Poison (death!) |", stylePoison)

	// Wall row
	r.putStr(x, y+5, "|  # Wall   (death!) |", styleWall)

	// Level progress
	threshold := int(r.level) * 5
	prog := fmt.Sprintf("|  Eat %2d for lvl up |", threshold)
	r.putStr(x, y+6, prog, styleUILabel)

	r.putStr(x, y+7, "+--------------------+", styleUILabel)
}

// ─────────────────────────────────────────────────────────────
// SCREEN STATES
// ─────────────────────────────────────────────────────────────

func (r *Renderer) DrawLobby(
	myName   string,
	players  []string,
	isHost   bool,
	mode     protocol.GameMode,
	duration uint16,
) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	type row struct {
		text  string
		style tcell.Style
	}
	var rows []row

	modeNames := map[protocol.GameMode]string{
		protocol.ModeSinglePlayer: "SINGLE PLAYER  -  SNAKE",
		protocol.ModeLastStanding: "MULTIPLAYER  -  LAST STANDING",
		protocol.ModeScoreRace:    "MULTIPLAYER  -  SCORE RACE",
	}
	title := modeNames[mode]
	if title == "" {
		title = "SNAKE"
	}

	rows = append(rows,
		row{"", styleDefault},
		row{title, styleUI},
		row{"", styleDefault},
		row{"-- players --", styleUILabel},
		row{"", styleDefault},
	)

	for i, p := range players {
		prefix := "   "
		if i == 0 {
			prefix = "[H] " // host marker, ASCII
		}
		col := snakeColors[i%len(snakeColors)]
		rows = append(rows, row{
			prefix + p,
			tcell.StyleDefault.Foreground(col).Bold(true),
		})
	}

	rows = append(rows, row{"", styleDefault})

	if isHost {
		switch mode {
		case protocol.ModeScoreRace:
			mins := duration / 60
			secs := duration % 60
			rows = append(rows,
				row{fmt.Sprintf("Race time: %02d:%02d  (+ add / - remove 30s)",
					mins, secs),
					tcell.StyleDefault.Foreground(tcell.ColorYellow)},
				row{"M  -  switch to Last Standing", styleUILabel},
				row{"", styleDefault},
			)
		default:
			if mode == protocol.ModeLastStanding {
				rows = append(rows,
					row{"M  -  switch to Score Race", styleUILabel},
					row{"", styleDefault},
				)
			}
		}
		rows = append(rows,
			row{"SPACE or ENTER  -  start", styleWin},
		)
	} else {
		if mode == protocol.ModeScoreRace {
			mins := duration / 60
			secs := duration % 60
			rows = append(rows,
				row{fmt.Sprintf("Race time: %02d:%02d", mins, secs), styleUILabel},
			)
		}
		rows = append(rows,
			row{"waiting for host to start...", styleUILabel},
		)
	}

	rows = append(rows,
		row{"", styleDefault},
		row{"Q to quit", styleUILabel},
	)

	startY := termH/2 - len(rows)/2
	if startY < 0 {
		startY = 0
	}
	for i, row := range rows {
		x := (termW - len(row.text)) / 2
		if x < 0 {
			x = 0
		}
		r.putStr(x, startY+i, row.text, row.style)
	}
	r.screen.Show()
}

func (r *Renderer) DrawGameOver(winnerID uint8, playerNames map[uint8]string) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	isSingle := r.gameMode == protocol.ModeSinglePlayer

	var msg   string
	var style tcell.Style

	switch {
	case winnerID == 0xFF && isSingle:
		msg   = "GAME OVER"
		style = styleDead
	case winnerID == r.playerID:
		msg   = "YOU WIN!"
		style = styleWin
	case winnerID == 0xFF:
		msg   = "DRAW  -  all snakes died"
		style = styleUI
	default:
		name := playerNames[winnerID]
		if name == "" {
			name = fmt.Sprintf("Player %d", winnerID)
		}
		msg   = fmt.Sprintf("GAME OVER  -  %s wins!", name)
		style = styleDead
	}

	cx := func(s string) int {
		x := (termW - len(s)) / 2
		if x < 0 {
			return 0
		}
		return x
	}

	y := termH/2 - 3
	r.putStr(cx(msg), y, msg, style)

	if winnerID == 0xFF && isSingle {
		sub := "you hit a wall or your own body"
		r.putStr(cx(sub), y+2, sub, styleUILabel)
	}

	r.putStr(cx("R  ->  play again"), y+4, "R  ->  play again", styleWin)
	r.putStr(cx("Q  ->  quit"), y+6, "Q  ->  quit", styleUILabel)
	r.screen.Show()
}

func (r *Renderer) DrawLevelComplete(level uint8) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	lines := []struct {
		t string
		s tcell.Style
	}{
		{fmt.Sprintf("LEVEL %d COMPLETE!", level), styleWin},
		{"", styleDefault},
		{fmt.Sprintf("advancing to level %d", level+1), styleUILabel},
		{"", styleDefault},
		{"SPACE to continue", styleUI},
		{"(auto-advances in 5 seconds)", styleUILabel},
	}

	sy := termH/2 - len(lines)/2
	for i, l := range lines {
		x := (termW - len(l.t)) / 2
		if x < 0 {
			x = 0
		}
		r.putStr(x, sy+i, l.t, l.s)
	}
	r.screen.Show()
}

func (r *Renderer) DrawGameComplete(winnerName string) {
	r.screen.Clear()
	termW, termH := r.screen.Size()

	lines := []struct {
		t string
		s tcell.Style
	}{
		{"ALL 10 LEVELS COMPLETE!", styleWin},
		{"", styleDefault},
		{"Champion: " + winnerName, styleUI},
		{"", styleDefault},
		{"R to play again   Q to quit", styleUI},
	}

	sy := termH/2 - len(lines)/2
	for i, l := range lines {
		x := (termW - len(l.t)) / 2
		if x < 0 {
			x = 0
		}
		r.putStr(x, sy+i, l.t, l.s)
	}
	r.screen.Show()
}

func (r *Renderer) DrawWaiting(name string) {
	r.screen.Clear()
	termW, termH := r.screen.Size()
	lines := []string{
		"SNAKE",
		"",
		"connected as: " + name,
		"",
		"connecting...",
	}
	sy := termH/2 - len(lines)/2
	for i, l := range lines {
		x := (termW - len(l)) / 2
		if x < 0 {
			x = 0
		}
		st := styleDefault
		if i == 0 {
			st = styleUI
		}
		r.putStr(x, sy+i, l, st)
	}
	r.screen.Show()
}

// ─────────────────────────────────────────────────────────────
// PRIMITIVE — putStr places a string char by char
// Never use fmt.Sprintf with emoji in fixed-width strings.
// Place emoji with SetContent directly.
// ─────────────────────────────────────────────────────────────

func (r *Renderer) putStr(x, y int, s string, style tcell.Style) {
	col := x
	for _, ch := range s {
		r.screen.SetContent(col, y, ch, nil, style)
		col++
	}
}