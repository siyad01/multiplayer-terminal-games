// internal/minesweeper/loop.go
// Minesweeper game loop — event-driven (no ticker).
// Reuses the same hub/channel infrastructure as Snake.

package minesweeper

import (
	"log"
	"math/rand"
	"time"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
	"github.com/siyad01/multiplayer-terminal-games/internal/snake"
)

// MineInputMsg carries a player action.
type MineInputMsg struct {
	PlayerID uint8
	Action   MineAction
	X, Y     uint8
}

type MineAction uint8

const (
	ActionReveal MineAction = iota
	ActionFlag
	ActionCursor
)

// Loop runs the Minesweeper game loop.
// Reuses snake.JoinMsg / snake.LeaveMsg / snake.StartMsg channels.
func Loop(
	mineCh      <-chan MineInputMsg,
	joinCh      <-chan snake.JoinMsg,
	leaveCh     <-chan snake.LeaveMsg,
	startCh     <-chan snake.StartMsg,
	setModeCh   <-chan SetModeMsg,
	stopCh      <-chan struct{},
	broadcastCh chan<- []byte,
	gameOverCh  chan<- uint8,
) {
	for {
		done := runOnce(
			mineCh, joinCh, leaveCh, startCh, setModeCh,
			stopCh, broadcastCh, gameOverCh,
		)
		if done {
			return
		}
		log.Println("minesweeper: restarting")
	}
}

type SetModeMsg struct {
	PlayerID   uint8
	Mode       Mode
	Difficulty string
}

func runOnce(
	mineCh      <-chan MineInputMsg,
	joinCh      <-chan snake.JoinMsg,
	leaveCh     <-chan snake.LeaveMsg,
	startCh     <-chan snake.StartMsg,
	setModeCh   <-chan SetModeMsg,
	stopCh      <-chan struct{},
	broadcastCh chan<- []byte,
	gameOverCh  chan<- uint8,
) bool {
	log.Println("minesweeper: lobby open")

	lobbyPlayers := make(map[uint8]string)
	lobbyIDs     := []uint8{}
	var hostID    uint8
	hasHost       := false
	gameMode      := ModeSolo
	difficulty    := "easy"

	broadcast := func(msg []byte) {
		select { case broadcastCh <- msg: default: }
	}

	addToLobby := func(id uint8, name string) bool {
		if len(lobbyPlayers) >= protocol.MaxPlayers { return false }
		if _, exists := lobbyPlayers[id]; exists { return true }
		lobbyPlayers[id] = name
		lobbyIDs = append(lobbyIDs, id)
		if !hasHost {
			hostID  = id
			hasHost = true
		}
		return true
	}

	broadcastLobby := func() {
		ids   := make([]uint8, 0, len(lobbyIDs))
		names := make([]string, 0, len(lobbyIDs))
		for _, id := range lobbyIDs {
			if n, ok := lobbyPlayers[id]; ok {
				ids   = append(ids, id)
				names = append(names, n)
			}
		}
		hid := uint8(0xFF)
		if hasHost { hid = hostID }
		// Reuse lobby state message — duration field = 0 for minesweeper
		broadcast(protocol.EncodeLobbyFull(hid, ids, names, 0,
			protocol.GameMode(gameMode+10))) // offset to distinguish from snake
	}

	// ── LOBBY ─────────────────────────────────────────────────
	for {
		select {
		case msg := <-joinCh:
			ok := addToLobby(msg.PlayerID, msg.Name)
			msg.ResultChan <- ok
			if ok { broadcastLobby() }

		case msg := <-leaveCh:
			delete(lobbyPlayers, msg.PlayerID)
			for i, id := range lobbyIDs {
				if id == msg.PlayerID {
					lobbyIDs = append(lobbyIDs[:i], lobbyIDs[i+1:]...)
					break
				}
			}
			if hasHost && msg.PlayerID == hostID && len(lobbyIDs) > 0 {
				hostID = lobbyIDs[0]
			}
			if len(lobbyPlayers) == 0 { hasHost = false }
			broadcastLobby()

		case msg := <-setModeCh:
			if !hasHost || msg.PlayerID != hostID { continue }
			gameMode   = msg.Mode
			difficulty = msg.Difficulty
			broadcastLobby()

		case msg := <-startCh:
			if !hasHost || msg.PlayerID != hostID { continue }
			if len(lobbyPlayers) == 0 { continue }
			log.Printf("minesweeper: starting mode=%d diff=%s players=%d",
				gameMode, difficulty, len(lobbyPlayers))
			goto startGame

		case <-stopCh:
			return true
		}
	}

startGame:
	diff := Difficulties[difficulty]
	if diff.Width == 0 { diff = Difficulties["easy"] }

	game := NewGame(diff, gameMode, rand.New(rand.NewSource(time.Now().UnixNano())))
	for _, id := range lobbyIDs {
		game.AddPlayer(id, lobbyPlayers[id])
		broadcast(protocol.EncodePlayerJoined(id, lobbyPlayers[id]))
	}

	// Send initial board state
	broadcast(protocol.EncodeMineState(
		game.Width, game.Height,
		game.VisibleCells(),
		game.Scores(),
		uint16(game.FlagsLeft),
	))

	log.Printf("minesweeper: board %dx%d mines=%d",
		game.Width, game.Height, game.MineCount)

	// ── GAME ──────────────────────────────────────────────────
	// No ticker — purely event-driven
	for {
		select {
		case msg := <-mineCh:
			if game.Over { continue }

			var result ActionResult
			var changes []protocol.MineChange

			switch msg.Action {
			case ActionReveal:
				result  = game.Reveal(msg.PlayerID, msg.X, msg.Y)
				changes = result.Changes

			case ActionFlag:
				changes = game.Flag(msg.PlayerID, msg.X, msg.Y)

			case ActionCursor:
				game.MoveCursor(msg.PlayerID, msg.X, msg.Y)
				// Broadcast cursor positions (cheap — just 4 bytes per player)
				broadcast(protocol.EncodeCursors(game.AllCursors()))
				continue
			}

			// Broadcast cell changes (partial update — efficient)
			if len(changes) > 0 {
				broadcast(protocol.EncodeMineUpdate(changes))
				// Also broadcast scores
				broadcast(protocol.EncodeMineState(
					game.Width, game.Height,
					game.VisibleCells(),
					game.Scores(),
					uint16(game.FlagsLeft),
				))
			}

			if result.Exploded {
				log.Printf("minesweeper: player %d hit a mine at (%d,%d)",
					msg.PlayerID, msg.X, msg.Y)
				broadcast(protocol.EncodeMineExplode(msg.PlayerID, msg.X, msg.Y))
				select { case gameOverCh <- msg.PlayerID: default: }
				time.Sleep(3 * time.Second)
				return false
			}

			if result.Won {
				log.Println("minesweeper: board cleared!")
				// Find highest scorer as winner
				var winner uint8 = 0xFF
				var best  uint16 = 0
				for _, p := range game.Players {
					if p.Reveals > best {
						best   = p.Reveals
						winner = p.ID
					}
				}
				broadcast(protocol.EncodeMineWin(game.Scores()))
				select { case gameOverCh <- winner: default: }
				time.Sleep(5 * time.Second)
				return false
			}

		case msg := <-joinCh:
			// Late join during game
			if gameMode == ModeCooperative || gameMode == ModeCompetitive {
				game.AddPlayer(msg.PlayerID, msg.Name)
				msg.ResultChan <- true
				// Send full board state to new player
				broadcast(protocol.EncodeMineState(
					game.Width, game.Height,
					game.VisibleCells(),
					game.Scores(),
					uint16(game.FlagsLeft),
				))
			} else {
				msg.ResultChan <- false
			}

		case msg := <-leaveCh:
			game.RemovePlayer(msg.PlayerID)
			log.Printf("minesweeper: player %d left", msg.PlayerID)

		case <-stopCh:
			return true
		}
	}
}