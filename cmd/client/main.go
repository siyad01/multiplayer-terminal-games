// cmd/client/main.go
// Unified client for all games.
// Connects to snake server (:8080) or minesweeper server (:8081)
// based on game selection in the main menu.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gorilla/websocket"
	"github.com/siyad01/multiplayer-terminal-games/internal/client"
	"github.com/siyad01/multiplayer-terminal-games/internal/minesweeper"
	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

const MinGameSeconds = uint16(120)

const (
	stLobby    = uint32(0)
	stPlaying  = uint32(1)
	stGameOver = uint32(2)
)

func main() {
	name := flag.String("name", "player", "your player name")
	flag.Parse()

	// ── Main menu ──────────────────────────────────────────────
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sel := showMainMenu(screen)
	screen.Fini()

	if sel.quit {
		return
	}

	// ── Route to correct game ──────────────────────────────────
	switch sel.game {
	case "snake":
		runSnake(*name, sel)
	case "mine":
		runMine(*name, sel)
	}
}

// ─────────────────────────────────────────────────────────────
// MENU SELECTION
// ─────────────────────────────────────────────────────────────

type Selection struct {
	quit       bool
	game       string // "snake" | "mine"
	mode       string // "single" | "multi"
	snakeGMode protocol.GameMode
	mineDiff   string // "easy" | "medium" | "hard"
	mineMode   minesweeper.Mode
}

func showMainMenu(screen tcell.Screen) Selection {
	styleTitle  := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
	styleSelect := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	styleNormal := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	styleSub    := tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleDim    := tcell.StyleDefault.Foreground(tcell.ColorGray)

	put := func(x, y int, s string, st tcell.Style) {
		for i, ch := range s {
			screen.SetContent(x+i, y, ch, nil, st)
		}
	}
	cx := func(s string) func(int) int {
		return func(w int) int {
			x := (w - len(s)) / 2
			if x < 0 { return 0 }
			return x
		}
	}

	type menuItem struct {
		label      string
		game       string
		mode       string
		snakeGMode protocol.GameMode
		mineMode   minesweeper.Mode
	}

	items := []menuItem{
		// ── Snake ─────────────────────────────────────────────
		{"Snake  -  Single Player",
			"snake", "single", protocol.ModeSinglePlayer, 0},
		{"Snake  -  Last Standing  (multiplayer)",
			"snake", "multi", protocol.ModeLastStanding, 0},
		{"Snake  -  Score Race     (multiplayer)",
			"snake", "multi", protocol.ModeScoreRace, 0},

		// ── Minesweeper ───────────────────────────────────────
		{"Minesweeper  -  Solo",
			"mine", "single", 0, minesweeper.ModeSolo},
		{"Minesweeper  -  Cooperative  (multiplayer)",
			"mine", "multi", 0, minesweeper.ModeCooperative},
		{"Minesweeper  -  Competitive  (multiplayer)",
			"mine", "multi", 0, minesweeper.ModeCompetitive},

		{"Quit", "", "", 0, 0},
	}

	diffs   := []string{"easy", "medium", "hard"}
	diffIdx := 0
	idx     := 0

	isMine := func() bool {
		return idx < len(items) && items[idx].game == "mine"
	}

	draw := func() {
		screen.Clear()
		w, h := screen.Size()

		title := "TERMINAL GAMES"
		put(cx(title)(w), h/2-10, title, styleTitle)

		hint := "arrows select   ENTER confirm   Q quit"
		put(cx(hint)(w), h/2-8, hint, styleDim)

		// Divider labels
		put(cx("-- SNAKE --")(w), h/2-6, "-- SNAKE --", styleSub)
		put(cx("-- MINESWEEPER --")(w), h/2-2, "-- MINESWEEPER --", styleSub)

		offsets := []int{-5, -4, -3, -1, 0, 1, 3}
		for i, item := range items {
			y  := h/2 + offsets[i]
			px := "  "
			st := styleNormal
			if i == idx {
				px = "> "
				st = styleSelect
			}
			if item.game == "" { // Quit
				y = h/2 + 4
			}
			put((w-44)/2, y, px+item.label, st)
		}

		// Difficulty selector — only shown when mine item selected
		if isMine() {
			dy := h/2 + 6
			put(cx("Difficulty:")(w), dy, "Difficulty:", styleSub)
			for i, d := range diffs {
				px := "  "
				st := styleNormal
				if i == diffIdx { px = "> "; st = styleSelect }
				put((w-20)/2, dy+1+i, px+d, st)
			}
			put(cx("TAB to select difficulty")(w), dy+5,
				"TAB cycles difficulty", styleDim)
		}

		screen.Show()
	}

	draw()
	for {
		ev := screen.PollEvent()
		if ev == nil { return Selection{quit: true} }
		switch e := ev.(type) {
		case *tcell.EventKey:
			switch e.Key() {
			case tcell.KeyUp:
				idx = (idx - 1 + len(items)) % len(items)
			case tcell.KeyDown:
				idx = (idx + 1) % len(items)
			case tcell.KeyTab:
				if isMine() {
					diffIdx = (diffIdx + 1) % len(diffs)
				}
			case tcell.KeyEnter:
				sel := items[idx]
				if sel.game == "" { return Selection{quit: true} }
				return Selection{
					game:       sel.game,
					mode:       sel.mode,
					snakeGMode: sel.snakeGMode,
					mineMode:   sel.mineMode,
					mineDiff:   diffs[diffIdx],
				}
			case tcell.KeyEscape:
				return Selection{quit: true}
			case tcell.KeyRune:
				switch e.Rune() {
				case 'q', 'Q':
					return Selection{quit: true}
				case '\t':
					if isMine() {
						diffIdx = (diffIdx + 1) % len(diffs)
					}
				}
			}
		case *tcell.EventResize:
			screen.Sync()
		}
		draw()
	}
}

// ─────────────────────────────────────────────────────────────
// SNAKE FLOW
// ─────────────────────────────────────────────────────────────

func runSnake(name string, sel Selection) {
	url := fmt.Sprintf("ws://localhost:8080/ws?name=%s", name)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snake server connection failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Is the snake server running? go run cmd/server/main.go")
		os.Exit(1)
	}
	defer conn.Close()

	renderer, err := client.NewRenderer(0, name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer renderer.Close()

	var playerID     atomic.Uint32
	playerID.Store(0xFF)
	var secondsLeft  atomic.Uint32
	var lobbyDuration atomic.Uint32
	lobbyDuration.Store(uint32(MinGameSeconds))
	var isHost       atomic.Bool
	var gameState    atomic.Uint32
	gameState.Store(stLobby)
	var currentMode  atomic.Uint32
	currentMode.Store(uint32(sel.snakeGMode))

	renderer.SetGameMode(sel.snakeGMode)

	playerNames := map[uint8]string{}

	done  := make(chan struct{})
	quit  := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Send initial mode to server after connecting
	go func() {
		time.Sleep(200 * time.Millisecond) // wait for welcome
		if sel.snakeGMode != protocol.ModeSinglePlayer {
			conn.WriteMessage(websocket.BinaryMessage,
				protocol.EncodeSetMode(sel.snakeGMode))
		}
	}()

	// ── Read server messages ───────────────────────────────────
	go func() {
		defer close(done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil { return }

			msgType, err := protocol.ParseType(data)
			if err != nil { continue }

			switch msgType {
			case protocol.MsgWelcome:
				if len(data) >= 2 {
					id := data[1]
					playerID.Store(uint32(id))
					renderer.UpdatePlayerID(id)
					playerNames[id] = name
				}

			case protocol.MsgLobbyState:
				gameState.Store(stLobby)
				if len(data) < 3 { continue }
				hostID := data[1]
				count  := int(data[2])
				myID   := uint8(playerID.Load())
				host   := myID == hostID
				isHost.Store(host)

				pList := []string{}
				for i := 0; i < count; i++ {
					base := 3 + i*(1+protocol.MaxNameLen)
					if base+1+protocol.MaxNameLen > len(data) { break }
					pid := data[base]
					raw := data[base+1 : base+1+protocol.MaxNameLen]
					n   := len(raw)
					for n > 0 && raw[n-1] == 0 { n-- }
					pname := string(raw[:n])
					playerNames[pid] = pname
					pList = append(pList, pname)
				}
				durBase := 3 + count*(1+protocol.MaxNameLen)
				dur     := uint16(MinGameSeconds)
				gmode   := sel.snakeGMode
				if durBase+3 <= len(data) {
					dur   = uint16(data[durBase])<<8 | uint16(data[durBase+1])
					gmode = protocol.GameMode(data[durBase+2])
				}
				if dur < MinGameSeconds { dur = MinGameSeconds }
				lobbyDuration.Store(uint32(dur))
				currentMode.Store(uint32(gmode))
				renderer.SetGameMode(gmode)
				renderer.DrawLobby(name, pList, host, gmode, dur)

				if sel.mode == "single" && host {
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodeStartGame())
				}

			case protocol.MsgPlayerJoined:
				id, pname, err := protocol.ParsePlayerEvent(data)
				if err != nil { continue }
				playerNames[id] = pname
				renderer.AddPlayer(id, pname)

			case protocol.MsgGameState:
				gameState.Store(stPlaying)
				w, h, _, board, scores, err := protocol.ParseGameState(data)
				if err != nil { continue }
				renderer.DrawGame(w, h, board, scores, playerNames,
					uint16(secondsLeft.Load()))

			case protocol.MsgTimerUpdate:
				secs, err := protocol.ParseTimerUpdate(data)
				if err != nil { continue }
				secondsLeft.Store(uint32(secs))

			case protocol.MsgLevelUp:
				if len(data) >= 2 { renderer.SetLevel(data[1]) }

			case protocol.MsgLevelComplete:
				if len(data) >= 2 { renderer.DrawLevelComplete(data[1]) }

			case protocol.MsgGameComplete:
				gameState.Store(stGameOver)
				if len(data) >= 2 {
					n := playerNames[data[1]]
					if n == "" { n = fmt.Sprintf("Player %d", data[1]) }
					renderer.DrawGameComplete(n)
				}

			case protocol.MsgGameOver:
				gameState.Store(stGameOver)
				if len(data) >= 2 {
					renderer.DrawGameOver(data[1], playerNames)
				}

			case protocol.MsgGameMode:
				if len(data) >= 2 {
					gm := protocol.GameMode(data[1])
					currentMode.Store(uint32(gm))
					renderer.SetGameMode(gm)
				}

			case protocol.MsgError:
				log.Printf("server: %s", protocol.ParseError(data))
			}
		}
	}()

	// ── Keyboard input ─────────────────────────────────────────
	go func() {
		for {
			ev := renderer.Screen().PollEvent()
			if ev == nil { return }
			switch e := ev.(type) {
			case *tcell.EventKey:
				kev := client.ParseKey(e)
				if kev.Quit { close(quit); return }

				state := gameState.Load()

				if state == stGameOver &&
					e.Key() == tcell.KeyRune &&
					(e.Rune() == 'r' || e.Rune() == 'R') {
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodePlayAgain())
					continue
				}

				if (e.Key() == tcell.KeyEnter ||
					(e.Key() == tcell.KeyRune && e.Rune() == ' ')) &&
					state == stLobby {
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodeStartGame())
				}

				if isHost.Load() && state == stLobby &&
					e.Key() == tcell.KeyRune {
					switch e.Rune() {
					case '+', '=':
						cur    := uint16(lobbyDuration.Load())
						newDur := cur + 30
						if newDur > 3600 { newDur = 3600 }
						conn.WriteMessage(websocket.BinaryMessage,
							protocol.EncodeSetTime(newDur))
					case '-', '_':
						cur := uint16(lobbyDuration.Load())
						if cur > MinGameSeconds {
							newDur := cur - 30
							if newDur < MinGameSeconds { newDur = MinGameSeconds }
							conn.WriteMessage(websocket.BinaryMessage,
								protocol.EncodeSetTime(newDur))
						}
					case 'm', 'M':
						cur := protocol.GameMode(currentMode.Load())
						var next protocol.GameMode
						switch cur {
						case protocol.ModeLastStanding:
							next = protocol.ModeScoreRace
						default:
							next = protocol.ModeLastStanding
						}
						conn.WriteMessage(websocket.BinaryMessage,
							protocol.EncodeSetMode(next))
					}
				}

				if kev.Dir != 0 && state == stPlaying {
					id := uint8(playerID.Load())
					if id == 0xFF { continue }
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodeInput(id, kev.Dir))
				}

			case *tcell.EventResize:
				renderer.Screen().Sync()
			}
		}
	}()

	select {
	case <-quit:
	case <-done:
		time.Sleep(2 * time.Second)
	case <-sigCh:
	}
	conn.WriteMessage(websocket.BinaryMessage, protocol.EncodeLeave())
}

// ─────────────────────────────────────────────────────────────
// MINESWEEPER FLOW
// ─────────────────────────────────────────────────────────────

func runMine(name string, sel Selection) {
	url := fmt.Sprintf("ws://localhost:8081/ws?name=%s&game=mine", name)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "minesweeper server connection failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Is the mine server running? go run cmd/mineserver/main.go")
		os.Exit(1)
	}
	defer conn.Close()

	scr, err := tcell.NewScreen()
	if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	if err := scr.Init(); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	defer scr.Fini()

	var playerID atomic.Uint32
	playerID.Store(0xFF)
	var gameState atomic.Uint32
	gameState.Store(stLobby)

	playerNames := map[uint8]string{}
	renderer    := minesweeper.NewRenderer(scr, 0, name)
	renderer.SetMode(sel.mineMode)

	var (
		lastWidth, lastHeight uint8
		lastCells             []protocol.MineCell
		lastScores            []uint16
		lastFlagsLeft         uint16
	)

	done  := make(chan struct{})
	quit  := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Send difficulty + mode after connecting
	go func() {
		time.Sleep(200 * time.Millisecond)
		conn.WriteMessage(websocket.BinaryMessage,
			protocol.EncodeSetMineMode(uint8(sel.mineMode), sel.mineDiff))
	}()

	// ── Read server messages ───────────────────────────────────
	go func() {
		defer close(done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil { return }
			msgType, err := protocol.ParseType(data)
			if err != nil { continue }

			switch msgType {
			case protocol.MsgWelcome:
				if len(data) >= 2 {
					id := data[1]
					playerID.Store(uint32(id))
					renderer = minesweeper.NewRenderer(scr, id, name)
					renderer.SetMode(sel.mineMode)
					playerNames[id] = name
				}

			case protocol.MsgLobbyState:
				gameState.Store(stLobby)
				if len(data) < 3 { continue }
				hostID := data[1]
				count  := int(data[2])
				myID   := uint8(playerID.Load())
				isHost := myID == hostID
				pList  := []string{}
				for i := 0; i < count; i++ {
					base := 3 + i*(1+protocol.MaxNameLen)
					if base+1+protocol.MaxNameLen > len(data) { break }
					pid := data[base]
					raw := data[base+1 : base+1+protocol.MaxNameLen]
					n   := len(raw)
					for n > 0 && raw[n-1] == 0 { n-- }
					pname := string(raw[:n])
					playerNames[pid] = pname
					pList = append(pList, pname)
				}
				renderer.DrawLobby(name, pList, isHost,
					sel.mineMode, sel.mineDiff)
				if sel.mineMode == minesweeper.ModeSolo && isHost {
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodeStartGame())
				}

			case protocol.MsgPlayerJoined:
				id, pname, err := protocol.ParsePlayerEvent(data)
				if err != nil { continue }
				playerNames[id] = pname
				renderer.AddPlayer(id, pname)

			case protocol.MsgMineState:
				w, h, flagsLeft, scores, cells, err :=
					protocol.ParseMineState(data)
				if err != nil { continue }
				gameState.Store(stPlaying)
				lastWidth, lastHeight = w, h
				lastCells, lastScores = cells, scores
				lastFlagsLeft         = flagsLeft
				renderer.DrawBoard(w, h, cells, scores,
					flagsLeft, playerNames)

			case protocol.MsgMineUpdate:
				changes, err := protocol.ParseMineUpdate(data)
				if err != nil { continue }
				if lastCells != nil {
					for _, c := range changes {
						idx := int(c.Y)*int(lastWidth) + int(c.X)
						if idx >= 0 && idx < len(lastCells) {
							lastCells[idx] = c.Cell
						}
					}
					renderer.DrawBoard(lastWidth, lastHeight,
						lastCells, lastScores, lastFlagsLeft, playerNames)
				}

			case protocol.MsgMineCursors:
				cursors, err := protocol.ParseCursors(data)
				if err != nil { continue }
				renderer.SetCursors(cursors)
				if lastCells != nil {
					renderer.DrawBoard(lastWidth, lastHeight,
						lastCells, lastScores, lastFlagsLeft, playerNames)
				}

			case protocol.MsgMineWin:
				gameState.Store(stGameOver)
				ns := 0
				if len(data) >= 2 { ns = int(data[1]) }
				scores := make([]uint16, ns)
				for i := range scores {
					if 2+i*2+1 < len(data) {
						scores[i] = uint16(data[2+i*2])<<8 |
							uint16(data[2+i*2+1])
					}
				}
				renderer.DrawWin(scores, playerNames)

			case protocol.MsgMineLose:
				gameState.Store(stGameOver)
				pid, x, y, err := protocol.ParseMineExplode(data)
				if err != nil { continue }
				renderer.DrawLose(pid, x, y, playerNames)

			case protocol.MsgError:
				log.Printf("server: %s", protocol.ParseError(data))
			}
		}
	}()

	// ── Keyboard input ─────────────────────────────────────────
	go func() {
		for {
			ev := scr.PollEvent()
			if ev == nil { return }
			switch e := ev.(type) {
			case *tcell.EventKey:
				state := gameState.Load()

				if e.Key() == tcell.KeyEscape ||
					(e.Key() == tcell.KeyRune &&
						(e.Rune() == 'q' || e.Rune() == 'Q')) {
					close(quit)
					return
				}

				if state == stGameOver &&
					e.Key() == tcell.KeyRune &&
					(e.Rune() == 'r' || e.Rune() == 'R') {
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodePlayAgain())
					continue
				}

				if state == stLobby {
					if e.Key() == tcell.KeyEnter ||
						(e.Key() == tcell.KeyRune && e.Rune() == ' ') {
						conn.WriteMessage(websocket.BinaryMessage,
							protocol.EncodeStartGame())
					}
					continue
				}

				if state != stPlaying || lastCells == nil { continue }

				cx := renderer.CursorX
				cy := renderer.CursorY
				w  := lastWidth
				h  := lastHeight
				id := uint8(playerID.Load())

				moved := false
				switch e.Key() {
				case tcell.KeyUp:
					if cy > 0 { renderer.CursorY--; moved = true }
				case tcell.KeyDown:
					if int(cy) < int(h)-1 { renderer.CursorY++; moved = true }
				case tcell.KeyLeft:
					if cx > 0 { renderer.CursorX--; moved = true }
				case tcell.KeyRight:
					if int(cx) < int(w)-1 { renderer.CursorX++; moved = true }
				case tcell.KeyEnter:
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodeReveal(renderer.CursorX,
							renderer.CursorY))
				}

				if e.Key() == tcell.KeyRune {
					switch e.Rune() {
					case 'f', 'F':
						conn.WriteMessage(websocket.BinaryMessage,
							protocol.EncodeFlag(renderer.CursorX,
								renderer.CursorY))
					case 'w', 'W':
						if cy > 0 { renderer.CursorY--; moved = true }
					case 's', 'S':
						if int(cy) < int(h)-1 { renderer.CursorY++; moved = true }
					case 'a', 'A':
						if cx > 0 { renderer.CursorX--; moved = true }
					case 'd', 'D':
						if int(cx) < int(w)-1 { renderer.CursorX++; moved = true }
					}
				}

				if moved {
					conn.WriteMessage(websocket.BinaryMessage,
						protocol.EncodeCursor(id,
							renderer.CursorX, renderer.CursorY))
					renderer.DrawBoard(lastWidth, lastHeight,
						lastCells, lastScores, lastFlagsLeft, playerNames)
				}

			case *tcell.EventResize:
				scr.Sync()
				if lastCells != nil {
					renderer.DrawBoard(lastWidth, lastHeight,
						lastCells, lastScores, lastFlagsLeft, playerNames)
				}
			}
		}
	}()

	select {
	case <-quit:
	case <-done:
		time.Sleep(2 * time.Second)
	case <-sigCh:
	}
	conn.WriteMessage(websocket.BinaryMessage, protocol.EncodeLeave())
}