// internal/protocol/mine_test.go

package protocol

import "testing"

func TestMineCellEncoding(t *testing.T) {
	// Revealed cells with adjacent counts 0-8
	for n := uint8(0); n <= 8; n++ {
		mc    := MineCell(0x30 + n)
		state := mc & 0xF0
		value := mc & 0x0F
		if state != 0x30 {
			t.Fatalf("n=%d: wrong state byte %x", n, state)
		}
		if value != MineCell(n) {
			t.Fatalf("n=%d: wrong value %d", n, value)
		}
	}
}

func TestMineStateRoundTrip(t *testing.T) {
	w, h      := uint8(9), uint8(9)
	cells     := make([]MineCell, int(w)*int(h))
	cells[0]   = MineCellFlagged
	cells[1]   = MineCell(0x30 + 3) // revealed, 3 adjacent mines
	cells[2]   = MineCellExploded

	scores    := []uint16{5, 12}
	flagsLeft := uint16(8)

	data := EncodeMineState(w, h, cells, scores, flagsLeft)

	rw, rh, rfl, rscores, rcells, err := ParseMineState(data)
	if err != nil {
		t.Fatalf("ParseMineState error: %v", err)
	}
	if rw != w || rh != h {
		t.Fatalf("dimensions: got %dx%d want %dx%d", rw, rh, w, h)
	}
	if rfl != flagsLeft {
		t.Fatalf("flagsLeft: got %d want %d", rfl, flagsLeft)
	}
	if len(rscores) != 2 || rscores[0] != 5 || rscores[1] != 12 {
		t.Fatalf("scores mismatch: got %v want [5 12]", rscores)
	}
	if rcells[0] != MineCellFlagged {
		t.Fatalf("cell[0]: got %x want %x", rcells[0], MineCellFlagged)
	}
	if rcells[1] != MineCell(0x30+3) {
		t.Fatalf("cell[1]: got %x want %x", rcells[1], MineCell(0x30+3))
	}
	if rcells[2] != MineCellExploded {
		t.Fatalf("cell[2]: got %x want %x", rcells[2], MineCellExploded)
	}
}

func TestMineUpdateRoundTrip(t *testing.T) {
	changes := []MineChange{
		{X: 3, Y: 4, Cell: MineCell(0x30 + 2)},
		{X: 0, Y: 0, Cell: MineCellFlagged},
		{X: 8, Y: 8, Cell: MineCellExploded},
	}

	data := EncodeMineUpdate(changes)
	got, err := ParseMineUpdate(data)
	if err != nil {
		t.Fatalf("ParseMineUpdate error: %v", err)
	}
	if len(got) != len(changes) {
		t.Fatalf("count: got %d want %d", len(got), len(changes))
	}
	for i, c := range changes {
		if got[i].X != c.X || got[i].Y != c.Y || got[i].Cell != c.Cell {
			t.Fatalf("change[%d]: got %+v want %+v", i, got[i], c)
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	cursors := []CursorPos{
		{PlayerID: 0, X: 3, Y: 7},
		{PlayerID: 1, X: 8, Y: 2},
	}
	data := EncodeCursors(cursors)
	got, err := ParseCursors(data)
	if err != nil {
		t.Fatalf("ParseCursors error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("cursor count: got %d want 2", len(got))
	}
	if got[0].X != 3 || got[0].Y != 7 {
		t.Fatalf("cursor[0]: %+v", got[0])
	}
	if got[1].PlayerID != 1 || got[1].X != 8 || got[1].Y != 2 {
		t.Fatalf("cursor[1]: %+v", got[1])
	}
}

func TestExplodeRoundTrip(t *testing.T) {
	data := EncodeMineExplode(2, 5, 9)
	pid, x, y, err := ParseMineExplode(data)
	if err != nil {
		t.Fatalf("ParseMineExplode error: %v", err)
	}
	if pid != 2 || x != 5 || y != 9 {
		t.Fatalf("explode: pid=%d x=%d y=%d want 2,5,9", pid, x, y)
	}
}

func TestSetMineModeRoundTrip(t *testing.T) {
	data := EncodeSetMineMode(1, "medium")
	mode, diff, err := ParseSetMineMode(data)
	if err != nil {
		t.Fatalf("ParseSetMineMode error: %v", err)
	}
	if mode != 1 {
		t.Fatalf("mode: got %d want 1", mode)
	}
	if diff != "medium" {
		t.Fatalf("diff: got %q want medium", diff)
	}
}