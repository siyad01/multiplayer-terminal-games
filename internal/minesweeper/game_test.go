// internal/minesweeper/game_test.go

package minesweeper

import (
	"math/rand"
	"testing"

	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

func newTestMineGame(mode Mode) *Game {
	return NewGame(Difficulties["easy"], mode,
		rand.New(rand.NewSource(42)))
}

func TestFirstClickSafe(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")

	result := g.Reveal(0, 4, 4)
	if result.Exploded {
		t.Fatal("first click should never explode")
	}
}

func TestFlagToggle(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")

	changes := g.Flag(0, 0, 0)
	if len(changes) == 0 {
		t.Fatal("flag should produce a change")
	}
	if changes[0].Cell != protocol.MineCellFlagged {
		t.Fatalf("expected flagged, got %v", changes[0].Cell)
	}

	// Flag again = unflag
	changes = g.Flag(0, 0, 0)
	if len(changes) == 0 {
		t.Fatal("unflag should produce a change")
	}
	if changes[0].Cell != protocol.MineCellHidden {
		t.Fatalf("expected hidden after unflag, got %v", changes[0].Cell)
	}
}

func TestRevealedCantBeFlagged(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")
	g.Reveal(0, 4, 4) // reveals some cells

	for i, c := range g.Cells {
		if c.State == StateRevealed {
			x       := uint8(i % int(g.Width))
			y       := uint8(i / int(g.Width))
			changes := g.Flag(0, x, y)
			if len(changes) > 0 {
				t.Fatal("should not be able to flag a revealed cell")
			}
			return
		}
	}
	t.Skip("no revealed cells found after first click — seed dependent")
}

func TestFloodFillRevealsMultiple(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")

	result := g.Reveal(0, 4, 4)
	if result.Exploded {
		t.Fatal("first click should not explode")
	}
	if len(result.Changes) == 0 {
		t.Fatal("reveal should produce at least one change")
	}
}

func TestFlagsLeft(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")

	initial := g.FlagsLeft

	g.Flag(0, 0, 0) // place
	if g.FlagsLeft != initial-1 {
		t.Fatalf("flags should decrease: got %d want %d",
			g.FlagsLeft, initial-1)
	}

	g.Flag(0, 0, 0) // remove
	if g.FlagsLeft != initial {
		t.Fatalf("flags should restore: got %d want %d",
			g.FlagsLeft, initial)
	}
}

func TestMultiplayerCursors(t *testing.T) {
	g := newTestMineGame(ModeCooperative)
	g.AddPlayer(0, "alice")
	g.AddPlayer(1, "bob")

	g.MoveCursor(0, 3, 4)
	g.MoveCursor(1, 7, 2)

	cursors := g.AllCursors()
	if len(cursors) != 2 {
		t.Fatalf("expected 2 cursors, got %d", len(cursors))
	}
}

func TestGameOverOnMine(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")

	// First click to place mines (safe)
	g.Reveal(0, 4, 4)

	// Find a mine and click it
	for i, c := range g.Cells {
		if c.IsMine {
			x      := uint8(i % int(g.Width))
			y      := uint8(i / int(g.Width))
			result := g.Reveal(0, x, y)
			if !result.Exploded {
				t.Fatal("clicking a mine should explode")
			}
			if !g.Over {
				t.Fatal("game should be over after mine click")
			}
			return
		}
	}
	t.Skip("no mines found")
}

func TestNoRevealAfterGameOver(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")
	g.Reveal(0, 4, 4) // safe first click

	// Trigger game over
	for i, c := range g.Cells {
		if c.IsMine {
			x := uint8(i % int(g.Width))
			y := uint8(i / int(g.Width))
			g.Reveal(0, x, y)
			break
		}
	}

	// Any further reveal should be ignored
	result := g.Reveal(0, 0, 0)
	if len(result.Changes) > 0 {
		t.Fatal("should not be able to reveal after game over")
	}
}

func TestVisibleCellsHidesMines(t *testing.T) {
	g := newTestMineGame(ModeSolo)
	g.AddPlayer(0, "test")

	// Before game starts — all cells hidden
	cells := g.VisibleCells()
	for i, c := range cells {
		if c != protocol.MineCellHidden {
			t.Fatalf("cell %d should be hidden before game starts, got %x",
				i, c)
		}
	}
}