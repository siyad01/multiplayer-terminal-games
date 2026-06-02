// internal/snake/loop.go

package snake

import (
	"log"
	"math/rand"
	"time"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)


type InputMsg struct {
	PlayerID uint8
	Dir protocol.Direction
}

type JoinMsg struct {
	PlayerID uint8
	Name string
	ResultChan chan bool
}

type LeaveMsg struct {
	PlayerID uint8
}
const (
	BoardWidth         = 45
	BoardHeight        = 20
	TickRate           = 180 * time.Millisecond
	DefaultRaceSeconds = 120
	MinRaceSeconds     = 120
)

type StartMsg   struct{ PlayerID uint8 }
type SetTimeMsg struct {
	PlayerID uint8
	Seconds  uint16
}
type SetModeMsg struct {
	PlayerID uint8
	Mode     protocol.GameMode
}
func Loop(
	inputCh     <-chan InputMsg,
	joinCh      <-chan JoinMsg,
	leaveCh     <-chan LeaveMsg,
	startCh     <-chan StartMsg,
	setTimeCh   <-chan SetTimeMsg,
	setModeCh   <-chan SetModeMsg,
	stopCh      <-chan struct{},
	broadcastCh chan<- []byte,
	gameOverCh  chan<- uint8,
) {
	survivors := map[uint8]string{}
	for {
		done := runOnce(
			inputCh, joinCh, leaveCh, startCh, setTimeCh, setModeCh,
			stopCh, broadcastCh, gameOverCh, survivors,
		)
		if done {
			return
		}
		log.Println("game loop: restarting")
	}
}

func runOnce(
	inputCh     <-chan InputMsg,
	joinCh      <-chan JoinMsg,
	leaveCh     <-chan LeaveMsg,
	startCh     <-chan StartMsg,
	setTimeCh   <-chan SetTimeMsg,
	setModeCh   <-chan SetModeMsg,
	stopCh      <-chan struct{},
	broadcastCh chan<- []byte,
	gameOverCh  chan<- uint8,
	survivors   map[uint8]string,
) bool {
	log.Println("game loop: lobby open")

	lobbyPlayers := make(map[uint8]string)
	lobbyIDs     := []uint8{}
	var hostID    uint8
	hasHost       := false
	gameMode      := protocol.ModeSinglePlayer
	raceDuration  := uint16(DefaultRaceSeconds)

	broadcast := func(msg []byte) {
		select { case broadcastCh <- msg: default: }
	}

	addToLobby := func(playerID uint8, name string) bool {
		if len(lobbyPlayers) >= protocol.MaxPlayers { return false }
		if _, exists := lobbyPlayers[playerID]; exists { return true }
		lobbyPlayers[playerID] = name
		lobbyIDs = append(lobbyIDs, playerID)
		if !hasHost {
			hostID  = playerID
			hasHost = true
			log.Printf("game loop: player %d (%s) is host", playerID, name)
		}
		return true
	}

	broadcastLobby := func() {
		ids   := make([]uint8, 0, len(lobbyIDs))
		names := make([]string, 0, len(lobbyIDs))
		for _, id := range lobbyIDs {
			if name, ok := lobbyPlayers[id]; ok {
				ids   = append(ids, id)
				names = append(names, name)
			}
		}
		hid := uint8(0xFF)
		if hasHost { hid = hostID }
		broadcast(protocol.EncodeLobbyFull(hid, ids, names, raceDuration, gameMode))
	}

	for id, name := range survivors {
		addToLobby(id, name)
		log.Printf("game loop: survivor %d (%s) re-entered lobby", id, name)
	}
	for k := range survivors { delete(survivors, k) }
	if len(lobbyPlayers) > 0 { 
		broadcastLobby()
		broadcast(protocol.EncodeGameMode(gameMode))
	}

	// ── LOBBY ─────────────────────────────────────────────────
	for {
		select {
		case msg := <-joinCh:
			ok := addToLobby(msg.PlayerID, msg.Name)
			if ok {
				log.Printf("game loop: player %d (%s) joined lobby (%d)",
					msg.PlayerID, msg.Name, len(lobbyPlayers))
				broadcastLobby()
			}
			msg.ResultChan <- ok

		case msg := <-leaveCh:
			delete(lobbyPlayers, msg.PlayerID)
			for i, id := range lobbyIDs {
				if id == msg.PlayerID {
					lobbyIDs = append(lobbyIDs[:i], lobbyIDs[i+1:]...)
					break
				}
			}
			if hasHost && msg.PlayerID == hostID {
				if len(lobbyIDs) > 0 {
					hostID = lobbyIDs[0]
					log.Printf("game loop: new host %d", hostID)
				} else {
					hasHost = false
				}
			}
			log.Printf("game loop: player %d left lobby", msg.PlayerID)
			broadcastLobby()

		case msg := <-setModeCh:
			if !hasHost || msg.PlayerID != hostID { continue }
			gameMode = msg.Mode
			log.Printf("game loop: mode set to %d", gameMode)
			broadcastLobby()

		case msg := <-setTimeCh:
			if !hasHost || msg.PlayerID != hostID { continue }
			// Only relevant for Score Race mode
			sec := msg.Seconds
			if sec < MinRaceSeconds { sec = MinRaceSeconds }
			if sec > 3600           { sec = 3600 }
			raceDuration = sec
			log.Printf("game loop: race duration set to %ds", raceDuration)
			broadcastLobby()

		case msg := <-startCh:
			if !hasHost || msg.PlayerID != hostID {
				log.Printf("game loop: player %d not host", msg.PlayerID)
				continue
			}
			if len(lobbyPlayers) == 0 { continue }
			log.Printf("game loop: starting — mode=%d players=%d",
				gameMode, len(lobbyPlayers))
			goto startGame

		case <-stopCh:
			return true
		}
	}

startGame:
	game := NewGame(BoardWidth, BoardHeight,
		rand.New(rand.NewSource(time.Now().UnixNano())))

	for _, id := range lobbyIDs {
		name := lobbyPlayers[id]
		game.AddPlayer(id, name)
		broadcast(protocol.EncodePlayerJoined(id, name))
		survivors[id] = name
	}

	// Broadcast mode so clients know what they're playing
	broadcast(protocol.EncodeGameMode(gameMode))
	broadcast(protocol.EncodeLevelUp(game.Level))

	ticker := time.NewTicker(TickRate)
	defer ticker.Stop()

	// Score Race: start countdown timer
	// Last Standing + Single Player: no timer
	var secondTicker *time.Ticker
	secondsLeft := uint16(0)
	if gameMode == protocol.ModeScoreRace {
		secondTicker = time.NewTicker(time.Second)
		defer secondTicker.Stop()
		secondsLeft = raceDuration
		broadcast(protocol.EncodeTimerUpdate(secondsLeft))
	}

	log.Printf("game loop: ticking level %d mode %d", game.Level, gameMode)

	for {
		// Build select cases dynamically based on mode
		if gameMode == protocol.ModeScoreRace && secondTicker != nil {
			select {
			case <-ticker.C:
				result := game.Tick()
				broadcast(protocol.EncodeGameState(
					uint8(game.Width), uint8(game.Height),
					game.Board(), game.Scores()))
				done := handleTickResult(result, game, gameMode,
					broadcast, gameOverCh, survivors)
				if done { return false }

			case <-secondTicker.C:
				if secondsLeft > 0 { secondsLeft-- }
				broadcast(protocol.EncodeTimerUpdate(secondsLeft))
				if secondsLeft == 0 {
					winner := highestScoreWinner(game)
					log.Printf("game loop: time up, winner %d", winner)
					select { case gameOverCh <- winner: default: }
					broadcast(protocol.EncodeGameOver(winner))
					time.Sleep(3 * time.Second)
					drainChannels(joinCh, leaveCh, startCh, setTimeCh, inputCh)
					return false
				}

			case msg := <-inputCh:
				game.SetDirection(msg.PlayerID, msg.Dir)
			case msg := <-joinCh:
				msg.ResultChan <- false
				broadcast(protocol.EncodeError("game in progress"))
			case msg := <-leaveCh:
				game.RemovePlayer(msg.PlayerID)
				delete(survivors, msg.PlayerID)
				broadcast(protocol.EncodePlayerLeft(msg.PlayerID, ""))
			case <-stopCh:
				return true
			}
		} else {
			// No timer: single player or last standing
			select {
			case <-ticker.C:
				result := game.Tick()
				broadcast(protocol.EncodeGameState(
					uint8(game.Width), uint8(game.Height),
					game.Board(), game.Scores()))

				switch result {
				case TickAllDead:
					winner := game.Winner()
					log.Printf("game loop: all dead, winner %d", winner)
					select { case gameOverCh <- winner: default: }
					broadcast(protocol.EncodeGameOver(winner))
					for _, p := range game.Players {
						if p.Status != StatusAlive {
							delete(survivors, p.ID)
						}
					}
					time.Sleep(3 * time.Second)
					drainChannels(joinCh, leaveCh, startCh, setTimeCh, inputCh)
					return false

				case TickLevelComplete:
					log.Printf("game loop: level %d complete", game.Level)
					broadcast(protocol.EncodeLevelComplete(game.Level, game.Winner()))
					waitForNext(startCh, stopCh, hostID)
					game.LevelUp()
					broadcast(protocol.EncodeLevelUp(game.Level))

				case TickGameComplete:
					winner := game.Winner()
					log.Printf("game loop: all levels done, winner %d", winner)
					broadcast(protocol.EncodeGameComplete(winner))
					select { case gameOverCh <- winner: default: }
					time.Sleep(5 * time.Second)
					drainChannels(joinCh, leaveCh, startCh, setTimeCh, inputCh)
					return false
				}

			case msg := <-inputCh:
				game.SetDirection(msg.PlayerID, msg.Dir)
			case msg := <-joinCh:
				msg.ResultChan <- false
				broadcast(protocol.EncodeError("game in progress"))
			case msg := <-leaveCh:
				game.RemovePlayer(msg.PlayerID)
				delete(survivors, msg.PlayerID)
				broadcast(protocol.EncodePlayerLeft(msg.PlayerID, ""))
			case <-stopCh:
				return true
			}
		}
	}
}

func handleTickResult(
	result TickResult,
	game *Game,
	mode protocol.GameMode,
	broadcast func([]byte),
	gameOverCh chan<- uint8,
	survivors map[uint8]string,
) bool {
	switch result {
	case TickAllDead:
		winner := game.Winner()
		select { case gameOverCh <- winner: default: }
		broadcast(protocol.EncodeGameOver(winner))
		for _, p := range game.Players {
			if p.Status != StatusAlive { delete(survivors, p.ID) }
		}
		return true
	}
	return false
}

func waitForNext(startCh <-chan StartMsg, stopCh <-chan struct{}, hostID uint8) {
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout: return
		case msg := <-startCh:
			if msg.PlayerID == hostID { return }
		case <-stopCh: return
		}
	}
}

func drainChannels(
	joinCh    <-chan JoinMsg,
	leaveCh   <-chan LeaveMsg,
	startCh   <-chan StartMsg,
	setTimeCh <-chan SetTimeMsg,
	inputCh   <-chan InputMsg,
) {
	for {
		select {
		case msg := <-joinCh: msg.ResultChan <- false
		case <-leaveCh:
		case <-startCh:
		case <-setTimeCh:
		case <-inputCh:
		default: return
		}
	}
}

func highestScoreWinner(game *Game) uint8 {
	var best *Player
	for _, p := range game.Players {
		if best == nil || p.Score > best.Score { best = p }
	}
	if best == nil { return 0xFF }
	return best.ID
}