package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,

	CheckOrigin: func(r *http.Request) bool {return true},
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "anonymous"
	}

	if len(name) > protocol.MaxNameLen {
		name = name[:protocol.MaxNameLen]
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	client := hub.NewClient(name)
	log.Printf("new connection: %s (assigned id=%d)", name, client.id)

	hub.Register(client)

	go client.writePump(conn)
	go client.readPump(conn)
}