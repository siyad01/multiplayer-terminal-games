// internal/server/hub.go — full replacement

package server

import (
	"log"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
	"github.com/siyad01/multiplayer-terminal-games/internal/snake"
)

type Client struct {
	id   uint8
	name string
	send chan []byte
	hub  *Hub
}

type registerMsg   struct{ client *Client }
type unregisterMsg struct{ client *Client }
type rejoinMsg     struct{ client *Client } // play again — no new connection

type Hub struct {
	clients    map[uint8]*Client
	register   chan registerMsg
	unregister chan unregisterMsg
	rejoin     chan rejoinMsg

	broadcastCh <-chan []byte
	inputCh     chan<- snake.InputMsg
	joinCh      chan<- snake.JoinMsg
	leaveCh     chan<- snake.LeaveMsg
	startCh     chan<- snake.StartMsg
	setTimeCh   chan<- snake.SetTimeMsg
	setModeCh chan<- snake.SetModeMsg

}

func NewHub(
	broadcastCh <-chan []byte,
	inputCh     chan<- snake.InputMsg,
	joinCh      chan<- snake.JoinMsg,
	leaveCh     chan<- snake.LeaveMsg,
	startCh     chan<- snake.StartMsg,
	setTimeCh   chan<- snake.SetTimeMsg,
	setModeCh chan<- snake.SetModeMsg,
) *Hub {
	return &Hub{
		clients:     make(map[uint8]*Client),
		register:    make(chan registerMsg, 16),
		unregister:  make(chan unregisterMsg, 16),
		rejoin:      make(chan rejoinMsg, 16),
		broadcastCh: broadcastCh,
		inputCh:     inputCh,
		joinCh:      joinCh,
		leaveCh:     leaveCh,
		startCh:     startCh,
		setTimeCh:   setTimeCh,
	}
}

func (h *Hub) assignID() (uint8, bool) {
	for id := uint8(0); id < protocol.MaxPlayers; id++ {
		if _, used := h.clients[id]; !used {
			return id, true
		}
	}
	return 0, false
}

func (h *Hub) sendToClient(c *Client, data []byte) bool {
	// Safe send — recover from send on closed channel
	defer func() { recover() }()
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

func (h *Hub) Run() {
	log.Println("hub started")
	for {
		select {

		case msg := <-h.register:
			c := msg.client
			id, ok := h.assignID()
			if !ok {
				// safely close — client goroutines haven't started yet
				close(c.send)
				log.Println("hub: rejected — server full")
				continue
			}
			c.id = id
			h.clients[id] = c
			log.Printf("hub: registered (id=%d name=%s total=%d)",
				c.id, c.name, len(h.clients))

			h.sendToClient(c, protocol.EncodeWelcome(c.id))

			resultCh := make(chan bool, 1)
			h.joinCh <- snake.JoinMsg{
				PlayerID:   c.id,
				Name:       c.name,
				ResultChan: resultCh,
			}
			go func(c *Client, resultCh chan bool) {
				ok := <-resultCh
				if !ok {
					h.sendToClient(c,
						protocol.EncodeError("game in progress — wait for next round"))
				}
			}(c, resultCh)

		case msg := <-h.rejoin:
			// Same client, same connection — just tell loop to add them to lobby
			c := msg.client
			if _, exists := h.clients[c.id]; !exists {
				// Client was already unregistered — ignore
				continue
			}
			log.Printf("hub: client %d rejoin lobby", c.id)
			resultCh := make(chan bool, 1)
			h.joinCh <- snake.JoinMsg{
				PlayerID:   c.id,
				Name:       c.name,
				ResultChan: resultCh,
			}
			go func(c *Client, resultCh chan bool) {
				<-resultCh // drain result, ignore value
			}(c, resultCh)

		case msg := <-h.unregister:
			c := msg.client
			existing, ok := h.clients[c.id]
			if !ok || existing != c {
				// Already replaced or removed — don't double-close
				log.Printf("hub: unregister ignored for stale client id=%d", c.id)
				continue
			}
			delete(h.clients, c.id)
			// Safe close with recover
			func() {
				defer recover()
				close(c.send)
			}()
			log.Printf("hub: unregistered (id=%d total=%d)", c.id, len(h.clients))
			h.leaveCh <- snake.LeaveMsg{PlayerID: c.id}

		case data := <-h.broadcastCh:
			for id, c := range h.clients {
				if !h.sendToClient(c, data) {
					log.Printf("hub: client %d too slow, dropping", id)
					delete(h.clients, id)
					func() {
						defer recover()
						close(c.send)
					}()
				}
			}
		}
	}
}

func (h *Hub) Register(c *Client)   { h.register <- registerMsg{client: c} }
func (h *Hub) Unregister(c *Client) { h.unregister <- unregisterMsg{client: c} }
func (h *Hub) Rejoin(c *Client)     { h.rejoin <- rejoinMsg{client: c} }

func (h *Hub) NewClient(name string) *Client {
	return &Client{
		id:   0xFF,
		name: name,
		send: make(chan []byte, 256),
		hub:  h,
	}
}

func (h *Hub) ForwardInput(playerID uint8, dir protocol.Direction) {
	select {
	case h.inputCh <- snake.InputMsg{PlayerID: playerID, Dir: dir}:
	default:
	}
}

func (h *Hub) ForwardStart(playerID uint8) {
	select {
	case h.startCh <- snake.StartMsg{PlayerID: playerID}:
	default:
	}
}

func (h *Hub) ForwardSetTime(playerID uint8, seconds uint16) {
	select {
	case h.setTimeCh <- snake.SetTimeMsg{PlayerID: playerID, Seconds: seconds}:
	default:
	}
}

// ForwardSetMode
func (h *Hub) ForwardSetMode(playerID uint8, mode protocol.GameMode) {
	select {
	case h.setModeCh <- snake.SetModeMsg{PlayerID: playerID, Mode: mode}:
	default:
	}
}

func (c *Client) ID() uint8        { return c.id }
func (c *Client) SendChan() <-chan []byte { return c.send }