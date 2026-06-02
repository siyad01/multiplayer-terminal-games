package server

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

const (
	writeWait = 10*time.Second

	pongWait = 60*time.Second

	pingPeriod = (pongWait*9)/10

	maxMessageSize = 512
)

func (c *Client) readPump(conn *websocket.Conn) {
	defer func ()  {
		c.hub.Unregister(c)
		conn.Close()
		log.Printf("readPump existing for client %d", c.id)
	}()

	conn.SetReadLimit(maxMessageSize)

	conn.SetReadDeadline(time.Now().Add(pongWait))

	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
			websocket.CloseGoingAway,
			websocket.CloseNormalClosure,
			websocket.CloseNoStatusReceived) {
				log.Printf("client %d unexpected close: %v", c.id, err)
			}
			return
		}

		msgType, err := protocol.ParseType(data)
		if err != nil {
			log.Printf("client %d bad message: %v", c.id, err)
			continue
		}

		switch msgType {
		case protocol.MsgInput:
			playerID, dir, err := protocol.ParseInput(data)
			if err != nil {
				log.Printf("client %d bad input: %v", c.id, err)
				continue
			}

			if playerID != c.id {
				log.Printf("client %d tried to send input as player %d", c.id, playerID)
				continue
			}
			c.hub.ForwardInput(c.id, dir)

		case protocol.MsgSetTime:
			seconds, err := protocol.ParseSetTime(data)
			if err != nil {
				log.Printf("client %d bad set time: %v", c.id, err)
				continue
			}
			c.hub.ForwardSetTime(c.id, seconds)
		case protocol.MsgSetMode:
			if len(data) >= 2 {
				c.hub.ForwardSetMode(c.id, protocol.GameMode(data[1]))
			}
		case protocol.MsgStartGame:
			// Only processed if this client is the host —
			// the game loop enforces that rule
			c.hub.ForwardStart(c.id)

		case protocol.MsgPlayAgain:
			// Client wants to re-enter the lobby after game over.
			// We re-register them — hub will assign them back into the lobby.
			log.Printf("client %d wants to play again", c.id)
			c.hub.Rejoin(c)

		case protocol.MsgNextLevel:
			c.hub.ForwardStart(c.id) // reuse start channel to signal "next level"
		case protocol.MsgLeave:
			log.Printf("client %d sent leave message", c.id)
			return

		default:
			log.Printf("client %d unknown message type: %x", c.id, msgType)
		}
	}
}

func (c *Client) writePump(conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func ()  {
		ticker.Stop()
		conn.Close()
		log.Printf("writePump existing for client %d", c.id)
	}()

	for {
		select {
		case data, ok := <-c.send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				log.Printf("client %d write error: %v", c.id, err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("client %d ping failed: %v", c.id, err)
				return
			}
		}
	}
}