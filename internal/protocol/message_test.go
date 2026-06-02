// messages_test.go — protocol correctness tests.
// Every encode must round-trip through its decode perfectly.
// If encode(x) → decode → y and x != y, the protocol is broken.

package protocol

import (
	"testing"
)

func TestJoinRoundTrip(t *testing.T) {
	name := "siyad"
	data := EncodeJoin(name)

	if MsgType(data[0]) != MsgJoin {
		t.Fatalf("wrong type byte: got %x want %x", data[0], MsgJoin)
	}

	got, err := ParseJoin(data)
	if err != nil {
		t.Fatalf("ParseJoin error: %v", err)
	}
	if got != name {
		t.Fatalf("name mismatch: got %q want %q", got, name)
	}
}

func TestInputRoundTrip(t *testing.T) {
	data := EncodeInput(2, DirLeft)

	pid, dir, err := ParseInput(data)
	if err != nil {
		t.Fatalf("ParseInput error: %v", err)
	}
	if pid != 2 {
		t.Fatalf("playerID mismatch: got %d want 2", pid)
	}
	if dir != DirLeft {
		t.Fatalf("direction mismatch: got %d want %d", dir, DirLeft)
	}
}

func TestGameStateRoundTrip(t *testing.T) {
	width, height := uint8(5), uint8(5)
	board := make([]Cell, int(width)*int(height))
	board[0] = CellHead1
	board[1] = CellSnake1
	board[12] = CellFood

	scores := []uint16{42, 7}

	data := EncodeGameState(width, height, board, scores)

	w, h, np, gotBoard, gotScores, err := ParseGameState(data)
	if err != nil {
		t.Fatalf("ParseGameState error: %v", err)
	}
	if w != width || h != height {
		t.Fatalf("dimensions mismatch: got %dx%d want %dx%d", w, h, width, height)
	}
	if np != 2 {
		t.Fatalf("numPlayers mismatch: got %d want 2", np)
	}
	if gotBoard[0] != CellHead1 {
		t.Fatalf("board[0] mismatch: got %d want %d", gotBoard[0], CellHead1)
	}
	if gotBoard[12] != CellFood {
		t.Fatalf("board[12] mismatch: got %d want %d", gotBoard[12], CellFood)
	}
	if gotScores[0] != 42 || gotScores[1] != 7 {
		t.Fatalf("scores mismatch: got %v want [42 7]", gotScores)
	}
}

func TestErrorRoundTrip(t *testing.T) {
	msg := "game is full"
	data := EncodeError(msg)
	got := ParseError(data)
	if got != msg {
		t.Fatalf("error mismatch: got %q want %q", got, msg)
	}
}

func TestInvalidInput(t *testing.T) {
	// Direction 0x99 is not valid — should return error
	data := []byte{byte(MsgInput), 0, 0x99}
	_, _, err := ParseInput(data)
	if err == nil {
		t.Fatal("expected error for invalid direction, got nil")
	}
}

func TestLongNameTruncated(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz" // 26 chars > MaxNameLen(15)
	data := EncodeJoin(long)
	got, err := ParseJoin(data)
	if err != nil {
		t.Fatalf("ParseJoin error: %v", err)
	}
	if len(got) > MaxNameLen {
		t.Fatalf("name not truncated: got len %d want <= %d", len(got), MaxNameLen)
	}
}