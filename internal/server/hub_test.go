// internal/server/hub_test.go
// Tests hub client registration, ID recycling, and broadcast.

package server_test

import (
	"testing"
	"time"
	"fmt"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
	"github.com/siyad01/multiplayer-terminal-games/internal/server"
	"github.com/siyad01/multiplayer-terminal-games/internal/snake"
)

// newTestHub creates a hub wired to buffered channels.
// All channels are drained by the test to prevent blocking.
func newTestHub() (
	*server.Hub,
	chan []byte,        // broadcastCh (server reads from this)
	chan snake.InputMsg,
	chan snake.JoinMsg,
	chan snake.LeaveMsg,
	chan snake.StartMsg,
	chan snake.SetTimeMsg,
	chan snake.SetModeMsg,
) {
	broadcastCh := make(chan []byte, 64)
	inputCh     := make(chan snake.InputMsg, 32)
	joinCh      := make(chan snake.JoinMsg, 8)
	leaveCh     := make(chan snake.LeaveMsg, 8)
	startCh     := make(chan snake.StartMsg, 4)
	setTimeCh   := make(chan snake.SetTimeMsg, 4)
	setModeCh   := make(chan snake.SetModeMsg, 4)

	h := server.NewHub(broadcastCh, inputCh, joinCh, leaveCh,
		startCh, setTimeCh, setModeCh)

	return h, broadcastCh, inputCh, joinCh, leaveCh,
		startCh, setTimeCh, setModeCh
}

// drainJoin reads and answers all pending JoinMsgs.
func drainJoin(joinCh chan snake.JoinMsg) {
	for {
		select {
		case msg := <-joinCh:
			msg.ResultChan <- true
		default:
			return
		}
	}
}

func TestHubRegisterAssignsLowestID(t *testing.T) {
	h, _, _, joinCh, leaveCh, _, _, _ := newTestHub()
	go h.Run()
	go func() {
		for msg := range joinCh {
			msg.ResultChan <- true
		}
	}()
	go func() {
		for range leaveCh {
		}
	}()

	c0 := h.NewClient("alice")
	c1 := h.NewClient("bob")
	h.Register(c0)
	h.Register(c1)

	time.Sleep(50 * time.Millisecond)

	if c0.ID() != 0 {
		t.Fatalf("first client should get ID 0, got %d", c0.ID())
	}
	if c1.ID() != 1 {
		t.Fatalf("second client should get ID 1, got %d", c1.ID())
	}
}

func TestHubIDRecycling(t *testing.T) {
	h, _, _, joinCh, leaveCh, _, _, _ := newTestHub()
	go h.Run()
	go func() {
		for msg := range joinCh { msg.ResultChan <- true }
	}()

	c0 := h.NewClient("alice")
	h.Register(c0)
	time.Sleep(30 * time.Millisecond)

	id0 := c0.ID()
	h.Unregister(c0)
	time.Sleep(30 * time.Millisecond)

	// Drain leave
	select {
	case <-leaveCh:
	default:
	}

	// New client should get ID 0 back
	c1 := h.NewClient("bob")
	h.Register(c1)
	time.Sleep(30 * time.Millisecond)

	if c1.ID() != id0 {
		t.Fatalf("recycled ID should be %d, got %d", id0, c1.ID())
	}
}

func TestHubBroadcastReachesAllClients(t *testing.T) {
	h, broadcastCh, _, joinCh, leaveCh, _, _, _ := newTestHub()
	go h.Run()
	go func() {
		for msg := range joinCh { msg.ResultChan <- true }
	}()
	go func() {
		for range leaveCh {}
	}()

	c0 := h.NewClient("alice")
	c1 := h.NewClient("bob")
	h.Register(c0)
	h.Register(c1)
	time.Sleep(50 * time.Millisecond)

	// Drain welcome messages from send channels
	drain := func(ch <-chan []byte) {
		for {
			select {
			case <-ch:
			default:
				return
			}
		}
	}
	time.Sleep(20 * time.Millisecond)
	drain(c0.SendChan())
	drain(c1.SendChan())

	// Send a broadcast
	msg := protocol.EncodeGameOver(0)
	broadcastCh <- msg

	time.Sleep(50 * time.Millisecond)

	// Both clients should receive it
	received := func(ch <-chan []byte) bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}

	if !received(c0.SendChan()) {
		t.Error("client 0 did not receive broadcast")
	}
	if !received(c1.SendChan()) {
		t.Error("client 1 did not receive broadcast")
	}
}

func TestHubMaxPlayers(t *testing.T) {
	h, _, _, joinCh, leaveCh, _, _, _ := newTestHub()
	go h.Run()
	go func() {
		for msg := range joinCh { msg.ResultChan <- true }
	}()
	go func() {
		for range leaveCh {}
	}()

	clients := make([]*server.Client, protocol.MaxPlayers+1)
	for i := range clients {
		clients[i] = h.NewClient(fmt.Sprintf("p%d", i))
		h.Register(clients[i])
	}
	time.Sleep(100 * time.Millisecond)

	// Count clients with valid IDs (0-3)
	valid := 0
	for _, c := range clients {
		if c.ID() < protocol.MaxPlayers {
			valid++
		}
	}
	if valid > protocol.MaxPlayers {
		t.Fatalf("hub accepted %d clients, max is %d", valid, protocol.MaxPlayers)
	}
}