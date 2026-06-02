// cmd/mineserver/main.go
// Minesweeper server — separate binary from snake server.
// Listens on :8081 to avoid conflict with snake server (:8080).

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/siyad01/multiplayer-terminal-games/internal/minesweeper"
	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
	"github.com/siyad01/multiplayer-terminal-games/internal/server"
	"github.com/siyad01/multiplayer-terminal-games/internal/snake"
)

func main() {
	// Shared channels
	broadcastCh := make(chan []byte, 64)
	joinCh := make(chan snake.JoinMsg, 4)
	leaveCh := make(chan snake.LeaveMsg, 4)
	startCh := make(chan snake.StartMsg, 4)
	gameOverCh := make(chan uint8, 1)
	stopCh := make(chan struct{})
	mineCh := make(chan minesweeper.MineInputMsg, 32)
	setModeCh := make(chan minesweeper.SetModeMsg, 4)

	// Start minesweeper loop
	go minesweeper.Loop(
		mineCh, joinCh, leaveCh, startCh, setModeCh,
		stopCh, broadcastCh, gameOverCh,
	)

	// Hub — reuse snake hub with mine message forwarding
	inputCh := make(chan snake.InputMsg, 1) // unused but hub needs it
	setTimeCh := make(chan snake.SetTimeMsg, 1)
	snakeModeCh := make(chan snake.SetModeMsg, 1) // unused in minesweeper but hub requires it
	hub := server.NewHub(broadcastCh, inputCh, joinCh, leaveCh,
		startCh, setTimeCh, snakeModeCh)
	go hub.Run()

	go func() {
		for id := range gameOverCh {
			log.Printf("minesweeper: game over, winner player %d", id)
		}
	}()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "anon"
		}
		if len(name) > protocol.MaxNameLen {
			name = name[:protocol.MaxNameLen]
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}

		c := hub.NewClient(name)
		hub.Register(c)

		// writePump: hub → WebSocket
		go func() {
			defer conn.Close()
			for data := range c.SendChan() {
				if err := conn.WriteMessage(
					websocket.BinaryMessage, data); err != nil {
					return
				}
			}
		}()

		// readPump: WebSocket → mine loop
		go func() {
			defer func() {
				hub.Unregister(c)
				conn.Close()
			}()
			for {
				_, data, err := conn.ReadMessage()
				if err != nil {
					return
				}

				msgType, err := protocol.ParseType(data)
				if err != nil {
					continue
				}

				switch msgType {
				case protocol.MsgMineReveal:
					if len(data) >= 3 {
						mineCh <- minesweeper.MineInputMsg{
							PlayerID: c.ID(),
							Action:   minesweeper.ActionReveal,
							X:        data[1],
							Y:        data[2],
						}
					}
				case protocol.MsgMineFlag:
					if len(data) >= 3 {
						mineCh <- minesweeper.MineInputMsg{
							PlayerID: c.ID(),
							Action:   minesweeper.ActionFlag,
							X:        data[1],
							Y:        data[2],
						}
					}
				case protocol.MsgMineCursor:
					if len(data) >= 4 {
						mineCh <- minesweeper.MineInputMsg{
							PlayerID: data[1],
							Action:   minesweeper.ActionCursor,
							X:        data[2],
							Y:        data[3],
						}
					}
				case protocol.MsgStartGame:
					hub.ForwardStart(c.ID())
				case protocol.MsgSetMineMode:
					mode, diff, err := protocol.ParseSetMineMode(data)
					if err != nil { continue }
					setModeCh <- minesweeper.SetModeMsg{
						PlayerID:   c.ID(),
						Mode:       minesweeper.Mode(mode),
						Difficulty: diff,
					}
				case protocol.MsgLeave:
					return
				case protocol.MsgPlayAgain:
					hub.Rejoin(c)
				}
			}
		}()
	})

	srv := &http.Server{Addr: ":8081", Handler: nil}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down minesweeper server...")
		close(stopCh)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("minesweeper server listening on :8081")
	log.Printf("connect: go run cmd/client/main.go -name yourname")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Println("minesweeper server stopped cleanly")

}
