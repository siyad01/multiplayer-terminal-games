package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/siyad01/multiplayer-terminal-games/internal/server"
	"github.com/siyad01/multiplayer-terminal-games/internal/snake"
)

func main() {
	broadcastCh := make(chan []byte, 64)
	inputCh     := make(chan snake.InputMsg, 32)
	joinCh      := make(chan snake.JoinMsg, 4)
	leaveCh     := make(chan snake.LeaveMsg, 4)
	startCh     := make(chan snake.StartMsg, 4)
	setTimeCh   := make(chan snake.SetTimeMsg, 4)
	setModeCh   := make(chan snake.SetModeMsg, 4)
	gameOverCh  := make(chan uint8, 1)
	stopCh      := make(chan struct{})

	go snake.Loop(
		inputCh, joinCh, leaveCh, startCh, setTimeCh, setModeCh,
		stopCh, broadcastCh, gameOverCh,
	)

	hub := server.NewHub(broadcastCh, inputCh, joinCh, leaveCh,
		startCh, setTimeCh, setModeCh)
	go hub.Run()

	go func() {
		for id := range gameOverCh {
			log.Printf("game over — winner: player %d", id)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWS(hub, w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("multiplayer-terminal-games snake server\n"))
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down snake server...")
		close(stopCh) // tell game loop to stop

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	addr := ":8080"
	log.Printf("snake server listening on %s", addr)
	log.Printf("connect: go run cmd/client/main.go -name yourname")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Println("snake server stopped cleanly")
}