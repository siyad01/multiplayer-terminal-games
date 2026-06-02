// internal/snake/game_test.go

package snake

import (
	"math/rand"
	"testing"
	"time"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

func newTestGame(w, h int) *Game {
	return NewGame(w, h, rand.New(rand.NewSource(42)))
}

// ── PROPERTY: new game has food on the board ──────────────────

func TestNewGameHasFood(t *testing.T) {
	g := newTestGame(20, 20)
	if len(g.Foods) == 0 {
		t.Fatal("new game should have at least one food")
	}
	for _, food := range g.Foods {
		if food.X < 0 || food.X >= 20 || food.Y < 0 || food.Y >= 20 {
			t.Fatalf("food out of bounds: %+v", food)
		}
	}
}

// ── PROPERTY: snake moves in its direction each tick ──────────

func TestSnakeMoves(t *testing.T) {
	g := newTestGame(20, 20)
	g.AddPlayer(0, "siyad")

	p            := g.Players[0]
	initialHead  := p.Body[0]

	// Place all food far away so snake won't eat
	g.Foods   = []Point{{19, 19}}
	g.Poisons = []Point{}

	p.Dir     = protocol.DirRight
	p.NextDir = protocol.DirRight

	g.Tick()

	newHead := p.Body[0]
	if newHead.X != initialHead.X+1 || newHead.Y != initialHead.Y {
		t.Fatalf("expected head at {%d,%d}, got {%d,%d}",
			initialHead.X+1, initialHead.Y, newHead.X, newHead.Y)
	}
}

// ── PROPERTY: snake grows when eating food ────────────────────

func TestSnakeGrowsOnFood(t *testing.T) {
	g := newTestGame(20, 20)
	g.AddPlayer(0, "test")

	p          := g.Players[0]
	initialLen := len(p.Body)

	// Place food directly in front of the snake head
	nextHead := step(p.Body[0], p.Dir)
	g.Foods   = []Point{nextHead}
	g.Poisons = []Point{}

	g.Tick()

	if len(p.Body) != initialLen+1 {
		t.Fatalf("snake should grow: got len %d want %d",
			len(p.Body), initialLen+1)
	}
	if p.Score != 1 {
		t.Fatalf("score should be 1 after eating: got %d", p.Score)
	}
}

// ── PROPERTY: snake dies on wall collision ────────────────────

func TestSnakeDiesOnWall(t *testing.T) {
	g := newTestGame(20, 20)
	g.AddPlayer(0, "test")

	p         := g.Players[0]
	p.Body[0]  = Point{0, 5}
	p.Dir      = protocol.DirLeft
	p.NextDir  = protocol.DirLeft
	g.Foods    = []Point{{19, 19}}
	g.Poisons  = []Point{}

	result := g.Tick()

	if p.Status != StatusDead {
		t.Fatal("snake should be dead after wall collision")
	}
	if result != TickAllDead {
		t.Fatalf("expected TickAllDead, got %d", result)
	}
}

// ── PROPERTY: 180 degree reversal is ignored ─────────────────

func TestNo180Reversal(t *testing.T) {
	g := newTestGame(20, 20)
	g.AddPlayer(0, "test")

	p         := g.Players[0]
	p.Dir      = protocol.DirRight
	p.NextDir  = protocol.DirRight

	g.SetDirection(0, protocol.DirLeft)

	if p.NextDir != protocol.DirRight {
		t.Fatal("180 degree reversal should be ignored")
	}
}

// ── PROPERTY: up to 4 players can join ───────────────────────

func TestMaxPlayers(t *testing.T) {
	g := newTestGame(20, 20)

	for i := 0; i < protocol.MaxPlayers; i++ {
		ok := g.AddPlayer(uint8(i), "player")
		if !ok {
			t.Fatalf("should be able to add player %d", i)
		}
	}

	ok := g.AddPlayer(4, "extra")
	if ok {
		t.Fatal("should not accept more than MaxPlayers players")
	}
}

// ── PROPERTY: board reflects game state ──────────────────────

func TestBoardReflectsState(t *testing.T) {
	g := newTestGame(20, 20)
	g.AddPlayer(0, "test")
	g.Foods   = []Point{{10, 10}}
	g.Poisons = []Point{}

	board := g.Board()

	foodIdx := 10*g.Width + 10
	if board[foodIdx] != protocol.CellFood {
		t.Fatalf("food not on board at expected position")
	}

	p       := g.Players[0]
	headIdx := p.Body[0].Y*g.Width + p.Body[0].X
	if board[headIdx] != protocol.CellHead1 {
		t.Fatalf("head not on board: got cell %d want %d",
			board[headIdx], protocol.CellHead1)
	}
}

// ── PROPERTY: score matches growth ───────────────────────────

func TestScoreMatchesGrowth(t *testing.T) {
	g := newTestGame(20, 20)
	g.AddPlayer(0, "test")
	p := g.Players[0]

	initialLen := len(p.Body)
	eaten      := 0

	for i := 0; i < 20; i++ {
		prevScore := p.Score
		result    := g.Tick()
		if result == TickAllDead {
			break
		}
		if p.Score > prevScore {
			eaten++
		}
	}

	expectedLen := initialLen + eaten
	if len(p.Body) != expectedLen {
		t.Fatalf("body length mismatch: got %d want %d (initial %d + eaten %d)",
			len(p.Body), expectedLen, initialLen, eaten)
	}
}

// ── PROPERTY: food eating works in all four directions ────────

func TestFoodEatingAllDirections(t *testing.T) {
	directions := []struct {
		name      string
		startDir  protocol.Direction // snake's initial direction
		moveDir   protocol.Direction // direction we want to test
		offset    Point              // food offset from head
	}{
		// For each test, startDir and moveDir must NOT be opposites
		{"right", protocol.DirRight, protocol.DirRight, Point{1, 0}},
		{"left",  protocol.DirLeft,  protocol.DirLeft,  Point{-1, 0}},
		{"down",  protocol.DirDown,  protocol.DirDown,  Point{0, 1}},
		{"up",    protocol.DirUp,    protocol.DirUp,    Point{0, -1}},
	}

	for _, tc := range directions {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGame(20, 20)

			// Place snake manually in center facing the right direction
			// so no 180° reversal issue
			body := []Point{
				{10, 10}, // head
				{10, 10}, // these get set by startDir below
				{10, 10},
			}
			// Build body behind the head based on start direction
			for i := 1; i < 3; i++ {
				body[i] = stepBack(body[0], tc.startDir, i)
			}

			g.Players = append(g.Players, &Player{
				ID:      0,
				Name:    "test",
				Body:    body,
				Dir:     tc.startDir,
				NextDir: tc.moveDir,
				Status:  StatusAlive,
				Level:   1,
			})

			head    := body[0]
			foodPos := Point{head.X + tc.offset.X, head.Y + tc.offset.Y}
			g.Foods   = []Point{foodPos}
			g.Poisons = []Point{}

			initialLen   := len(body)
			initialScore := g.Players[0].Score

			result := g.Tick()

			p := g.Players[0]
			if result == TickAllDead {
				t.Fatalf("direction %s: snake died. head was %v, food was %v",
					tc.name, head, foodPos)
			}
			if p.Score != initialScore+1 {
				t.Fatalf("direction %s: score not incremented. head was %v, food was %v, new head is %v",
					tc.name, head, foodPos, p.Body[0])
			}
			if len(p.Body) != initialLen+1 {
				t.Fatalf("direction %s: snake did not grow. len=%d want=%d",
					tc.name, len(p.Body), initialLen+1)
			}
		})
	}
}

// ── LOOP TESTS ────────────────────────────────────────────────

func TestLoopTicksBroadcast(t *testing.T) {
	inputCh     := make(chan InputMsg, 10)
	joinCh      := make(chan JoinMsg, 4)
	leaveCh     := make(chan LeaveMsg, 4)
	startCh     := make(chan StartMsg, 4)
	setTimeCh   := make(chan SetTimeMsg, 4)
	setModeCh   := make(chan SetModeMsg, 4)
	stopCh      := make(chan struct{})
	broadcastCh := make(chan []byte, 100)
	gameOverCh  := make(chan uint8, 1)

	go Loop(inputCh, joinCh, leaveCh, startCh, setTimeCh, setModeCh,
		stopCh, broadcastCh, gameOverCh)

	resultCh := make(chan bool, 1)
	joinCh <- JoinMsg{PlayerID: 0, Name: "test", ResultChan: resultCh}
	if !<-resultCh {
		t.Fatal("player should be able to join")
	}

	startCh <- StartMsg{PlayerID: 0}

	count   := 0
	timeout := time.After(2 * time.Second)
	for count < 3 {
		select {
		case msg := <-broadcastCh:
			if len(msg) == 0 {
				t.Fatal("received empty broadcast")
			}
			count++
		case <-timeout:
			t.Fatalf("timeout waiting for broadcasts, got %d", count)
		}
	}

	close(stopCh)
	time.Sleep(50 * time.Millisecond)
}

func TestLoopInputProcessed(t *testing.T) {
	inputCh     := make(chan InputMsg, 10)
	joinCh      := make(chan JoinMsg, 4)
	leaveCh     := make(chan LeaveMsg, 4)
	startCh     := make(chan StartMsg, 4)
	setTimeCh   := make(chan SetTimeMsg, 4)
	setModeCh   := make(chan SetModeMsg, 4)
	stopCh      := make(chan struct{})
	broadcastCh := make(chan []byte, 100)
	gameOverCh  := make(chan uint8, 1)

	go Loop(inputCh, joinCh, leaveCh, startCh, setTimeCh, setModeCh,
		stopCh, broadcastCh, gameOverCh)

	resultCh := make(chan bool, 1)
	joinCh <- JoinMsg{PlayerID: 0, Name: "inputtest", ResultChan: resultCh}
	<-resultCh

	startCh <- StartMsg{PlayerID: 0}

	inputCh <- InputMsg{PlayerID: 0, Dir: protocol.DirUp}
	inputCh <- InputMsg{PlayerID: 0, Dir: protocol.DirRight}

	time.Sleep(200 * time.Millisecond)
	close(stopCh)
}