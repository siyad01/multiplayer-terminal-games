// internal/protocol/messages.go — complete file

package protocol

import (
	"encoding/binary"
	"fmt"
)

type MsgType uint8

const (
	// Client → Server
	MsgJoin      MsgType = 0x01
	MsgInput     MsgType = 0x02
	MsgLeave     MsgType = 0x03
	MsgStartGame MsgType = 0x04
	MsgSetTime   MsgType = 0x05
	MsgPlayAgain MsgType = 0x06
	MsgNextLevel MsgType = 0x07
	MsgSetMode   MsgType = 0x08

	// Server → Client
	MsgGameState    MsgType = 0x10
	MsgPlayerJoined MsgType = 0x11
	MsgPlayerLeft   MsgType = 0x12
	MsgGameOver     MsgType = 0x13
	MsgWelcome      MsgType = 0x14
	MsgLobbyState   MsgType = 0x15
	MsgTimerUpdate  MsgType = 0x16
	MsgLevelUp      MsgType = 0x17
	MsgLevelComplete MsgType = 0x18
	MsgGameComplete MsgType = 0x19
	MsgGameMode     MsgType = 0x1A
	MsgError        MsgType = 0xFF

	// ── Minesweeper Client → Server ───────────────────────────────
	MsgMineReveal  MsgType = 0x20
	MsgMineFlag    MsgType = 0x21
	MsgMineCursor  MsgType = 0x22

	// ── Minesweeper Server → Client ───────────────────────────────
	MsgMineState   MsgType = 0x30
	MsgMineUpdate  MsgType = 0x31
	MsgMineWin     MsgType = 0x32
	MsgMineLose    MsgType = 0x33
	MsgMineCursors MsgType = 0x34

	MsgSetMineMode MsgType = 0x09 // client → server: mine mode + difficulty

)

type Direction uint8

const (
	DirUp    Direction = 0x01
	DirDown  Direction = 0x02
	DirLeft  Direction = 0x03
	DirRight Direction = 0x04
)

type Cell uint8

const (
	CellEmpty  Cell = 0x00
	CellFood   Cell = 0x01
	CellSnake1 Cell = 0x02
	CellSnake2 Cell = 0x03
	CellSnake3 Cell = 0x04
	CellSnake4 Cell = 0x05
	CellHead1  Cell = 0x06
	CellHead2  Cell = 0x07
	CellHead3  Cell = 0x08
	CellHead4  Cell = 0x09
	CellWall   Cell = 0x0A
	CellPoison Cell = 0x0B
)

type GameMode uint8

const (
	ModeSinglePlayer GameMode = 0x01
	ModeLastStanding GameMode = 0x02
	ModeScoreRace    GameMode = 0x03
)

const (
	MaxNameLen = 15
	MaxPlayers = 4
)

// ── Encode ────────────────────────────────────────────────────

func EncodeJoin(name string) []byte {
	buf := make([]byte, 1+MaxNameLen)
	buf[0] = byte(MsgJoin)
	copy(buf[1:], []byte(name))
	return buf
}

func EncodeInput(playerID uint8, dir Direction) []byte {
	return []byte{byte(MsgInput), playerID, byte(dir)}
}

func EncodeLeave() []byte { return []byte{byte(MsgLeave)} }

func EncodeStartGame() []byte { return []byte{byte(MsgStartGame)} }

func EncodePlayAgain() []byte { return []byte{byte(MsgPlayAgain)} }

func EncodeNextLevel() []byte { return []byte{byte(MsgNextLevel)} }

func EncodeSetMode(mode GameMode) []byte {
	return []byte{byte(MsgSetMode), byte(mode)}
}

func EncodeSetTime(seconds uint16) []byte {
	buf := make([]byte, 3)
	buf[0] = byte(MsgSetTime)
	binary.BigEndian.PutUint16(buf[1:], seconds)
	return buf
}

func EncodeGameState(width, height uint8, board []Cell, scores []uint16) []byte {
	numPlayers := uint8(len(scores))
	size := 4 + int(width)*int(height) + int(numPlayers)*2
	buf := make([]byte, size)
	buf[0] = byte(MsgGameState)
	buf[1] = width
	buf[2] = height
	buf[3] = numPlayers
	base := 4
	for i, cell := range board {
		buf[base+i] = byte(cell)
	}
	scoreBase := base + int(width)*int(height)
	for i, score := range scores {
		binary.BigEndian.PutUint16(buf[scoreBase+i*2:], score)
	}
	return buf
}

func EncodePlayerJoined(playerID uint8, name string) []byte {
	buf := make([]byte, 2+MaxNameLen)
	buf[0] = byte(MsgPlayerJoined)
	buf[1] = playerID
	copy(buf[2:], []byte(name))
	return buf
}

func EncodePlayerLeft(playerID uint8, name string) []byte {
	buf := make([]byte, 2+MaxNameLen)
	buf[0] = byte(MsgPlayerLeft)
	buf[1] = playerID
	copy(buf[2:], []byte(name))
	return buf
}

func EncodeGameOver(winnerID uint8) []byte {
	return []byte{byte(MsgGameOver), winnerID}
}

func EncodeWelcome(playerID uint8) []byte {
	return []byte{byte(MsgWelcome), playerID}
}

func EncodeTimerUpdate(secondsLeft uint16) []byte {
	buf := make([]byte, 3)
	buf[0] = byte(MsgTimerUpdate)
	binary.BigEndian.PutUint16(buf[1:], secondsLeft)
	return buf
}

func EncodeLevelUp(level uint8) []byte {
	return []byte{byte(MsgLevelUp), level}
}

func EncodeLevelComplete(level uint8, winnerID uint8) []byte {
	return []byte{byte(MsgLevelComplete), level, winnerID}
}

func EncodeGameComplete(winnerID uint8) []byte {
	return []byte{byte(MsgGameComplete), winnerID}
}

func EncodeGameMode(mode GameMode) []byte {
	return []byte{byte(MsgGameMode), byte(mode)}
}

func EncodeError(msg string) []byte {
	buf := make([]byte, 1+len(msg))
	buf[0] = byte(MsgError)
	copy(buf[1:], []byte(msg))
	return buf
}

// EncodeLobbyFull encodes lobby state + duration + mode.
// Format: type hostID count [id name*15]... durationHi durationLo mode
func EncodeLobbyFull(
	hostID   uint8,
	ids      []uint8,
	names    []string,
	duration uint16,
	mode     GameMode,
) []byte {
	count := len(ids)
	buf   := make([]byte, 3+count*(1+MaxNameLen)+3)
	buf[0] = byte(MsgLobbyState)
	buf[1] = hostID
	buf[2] = uint8(count)
	for i := 0; i < count; i++ {
		base := 3 + i*(1+MaxNameLen)
		buf[base] = ids[i]
		copy(buf[base+1:base+1+MaxNameLen], []byte(names[i]))
	}
	durBase := 3 + count*(1+MaxNameLen)
	binary.BigEndian.PutUint16(buf[durBase:], duration)
	buf[durBase+2] = byte(mode)
	return buf
}

// ── Decode ────────────────────────────────────────────────────

func ParseType(data []byte) (MsgType, error) {
	if len(data) < 1 {
		return 0, fmt.Errorf("empty message")
	}
	return MsgType(data[0]), nil
}

func ParseJoin(data []byte) (string, error) {
	if len(data) < 2 {
		return "", fmt.Errorf("join too short")
	}
	raw := data[1:]
	if len(raw) > MaxNameLen {
		raw = raw[:MaxNameLen]
	}
	n := len(raw)
	for n > 0 && raw[n-1] == 0 {
		n--
	}
	return string(raw[:n]), nil
}

func ParseInput(data []byte) (uint8, Direction, error) {
	if len(data) < 3 {
		return 0, 0, fmt.Errorf("input too short")
	}
	dir := Direction(data[2])
	if dir < DirUp || dir > DirRight {
		return 0, 0, fmt.Errorf("invalid direction: %d", dir)
	}
	return data[1], dir, nil
}

func ParseGameState(data []byte) (width, height, numPlayers uint8, board []Cell, scores []uint16, err error) {
	if len(data) < 4 {
		return 0, 0, 0, nil, nil, fmt.Errorf("game state too short")
	}
	width      = data[1]
	height     = data[2]
	numPlayers = data[3]
	boardSize  := int(width) * int(height)
	if len(data) < 4+boardSize+int(numPlayers)*2 {
		return 0, 0, 0, nil, nil, fmt.Errorf("game state truncated")
	}
	board = make([]Cell, boardSize)
	for i := range board {
		board[i] = Cell(data[4+i])
	}
	scores = make([]uint16, numPlayers)
	sb := 4 + boardSize
	for i := range scores {
		scores[i] = binary.BigEndian.Uint16(data[sb+i*2:])
	}
	return width, height, numPlayers, board, scores, nil
}

func ParseSetTime(data []byte) (uint16, error) {
	if len(data) < 3 {
		return 0, fmt.Errorf("set time too short")
	}
	return binary.BigEndian.Uint16(data[1:]), nil
}

func ParseTimerUpdate(data []byte) (uint16, error) {
	if len(data) < 3 {
		return 0, fmt.Errorf("timer update too short")
	}
	return binary.BigEndian.Uint16(data[1:]), nil
}

func ParseError(data []byte) string {
	if len(data) < 2 {
		return "unknown error"
	}
	return string(data[1:])
}

func ParsePlayerEvent(data []byte) (uint8, string, error) {
	if len(data) < 3 {
		return 0, "", fmt.Errorf("player event too short")
	}
	raw := data[2:]
	n   := len(raw)
	for n > 0 && raw[n-1] == 0 {
		n--
	}
	return data[1], string(raw[:n]), nil
}


func EncodeSetMineMode(mode uint8, diff string) []byte {
	buf := make([]byte, 2+MaxNameLen)
	buf[0] = byte(MsgSetMineMode)
	buf[1] = mode
	copy(buf[2:], []byte(diff))
	return buf
}

func ParseSetMineMode(data []byte) (mode uint8, diff string, err error) {
	if len(data) < 3 {
		return 0, "", fmt.Errorf("set mine mode too short")
	}
	mode = data[1]
	raw  := data[2:]
	n    := len(raw)
	for n > 0 && raw[n-1] == 0 { n-- }
	return mode, string(raw[:n]), nil
}
// ── Minesweeper encode ────────────────────────────────────────

// EncodeReveal: [type][x uint8][y uint8]
func EncodeReveal(x, y uint8) []byte {
	return []byte{byte(MsgMineReveal), x, y}
}

// EncodeFlag: [type][x uint8][y uint8]
func EncodeFlag(x, y uint8) []byte {
	return []byte{byte(MsgMineFlag), x, y}
}

// EncodeCursor: [type][playerID][x][y]
func EncodeCursor(playerID, x, y uint8) []byte {
	return []byte{byte(MsgMineCursor), playerID, x, y}
}

// MineCell encodes one cell's visible state for the wire.
// Bits: [7-4 = state][3-0 = value]
// state: 0=hidden 1=revealed 2=flagged 3=exploded
// value: 0-8 adjacent mines, 9=mine
type MineCell uint8

const (
	MineCellHidden   MineCell = 0x00
	MineCellFlagged  MineCell = 0x10
	MineCellExploded MineCell = 0x20
	// Revealed: 0x30 + adjacent count (0-8)
	MineCellRevealed0 MineCell = 0x30
	// MineCell(0x30 + n) for n adjacent mines
)

func EncodeMineState(
	width, height uint8,
	cells []MineCell,
	scores []uint16, // per-player reveal count
	flagsLeft uint16,
) []byte {
	// [type][w][h][flagsHi][flagsLo][numScores]
	// [scores: 2 bytes each]
	// [cells: 1 byte each, row-major]
	ns   := uint8(len(scores))
	size := 6 + int(ns)*2 + int(width)*int(height)
	buf  := make([]byte, size)
	buf[0] = byte(MsgMineState)
	buf[1] = width
	buf[2] = height
	binary.BigEndian.PutUint16(buf[3:], flagsLeft)
	buf[5] = ns
	base := 6
	for i, s := range scores {
		binary.BigEndian.PutUint16(buf[base+i*2:], s)
	}
	base += int(ns) * 2
	for i, c := range cells {
		buf[base+i] = byte(c)
	}
	return buf
}

func ParseMineState(data []byte) (
	width, height uint8,
	flagsLeft uint16,
	scores []uint16,
	cells []MineCell,
	err error,
) {
	if len(data) < 6 {
		return 0, 0, 0, nil, nil, fmt.Errorf("mine state too short")
	}
	width     = data[1]
	height    = data[2]
	flagsLeft = binary.BigEndian.Uint16(data[3:])
	ns        := int(data[5])
	base      := 6
	if len(data) < base+ns*2+int(width)*int(height) {
		return 0, 0, 0, nil, nil, fmt.Errorf("mine state truncated")
	}
	scores = make([]uint16, ns)
	for i := range scores {
		scores[i] = binary.BigEndian.Uint16(data[base+i*2:])
	}
	base += ns * 2
	cells = make([]MineCell, int(width)*int(height))
	for i := range cells {
		cells[i] = MineCell(data[base+i])
	}
	return width, height, flagsLeft, scores, cells, nil
}

// EncodeMineUpdate sends only changed cells.
// Format: [type][count uint16][x][y][cell] × count
func EncodeMineUpdate(changes []MineChange) []byte {
	buf := make([]byte, 3+len(changes)*3)
	buf[0] = byte(MsgMineUpdate)
	binary.BigEndian.PutUint16(buf[1:], uint16(len(changes)))
	for i, c := range changes {
		buf[3+i*3]   = c.X
		buf[3+i*3+1] = c.Y
		buf[3+i*3+2] = byte(c.Cell)
	}
	return buf
}

type MineChange struct {
	X, Y uint8
	Cell MineCell
}

func ParseMineUpdate(data []byte) ([]MineChange, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("mine update too short")
	}
	count   := int(binary.BigEndian.Uint16(data[1:]))
	changes := make([]MineChange, count)
	if len(data) < 3+count*3 {
		return nil, fmt.Errorf("mine update truncated")
	}
	for i := range changes {
		changes[i] = MineChange{
			X:    data[3+i*3],
			Y:    data[3+i*3+1],
			Cell: MineCell(data[3+i*3+2]),
		}
	}
	return changes, nil
}

func EncodeMineWin(scores []uint16) []byte {
	buf := make([]byte, 3+len(scores)*2)
	buf[0] = byte(MsgMineWin)
	buf[1] = uint8(len(scores))
	for i, s := range scores {
		binary.BigEndian.PutUint16(buf[2+i*2:], s)
	}
	return buf
}

func EncodeMineExplode(playerID, x, y uint8) []byte {
	return []byte{byte(MsgMineLose), playerID, x, y}
}

func ParseMineExplode(data []byte) (playerID, x, y uint8, err error) {
	if len(data) < 4 {
		return 0, 0, 0, fmt.Errorf("explode msg too short")
	}
	return data[1], data[2], data[3], nil
}

// EncodeCursors sends all player cursor positions.
// Format: [type][count][playerID x y] × count
func EncodeCursors(cursors []CursorPos) []byte {
	buf := make([]byte, 2+len(cursors)*3)
	buf[0] = byte(MsgMineCursors)
	buf[1] = uint8(len(cursors))
	for i, c := range cursors {
		buf[2+i*3]   = c.PlayerID
		buf[2+i*3+1] = c.X
		buf[2+i*3+2] = c.Y
	}
	return buf
}

type CursorPos struct {
	PlayerID, X, Y uint8
}

func ParseCursors(data []byte) ([]CursorPos, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("cursors too short")
	}
	count   := int(data[1])
	cursors := make([]CursorPos, count)
	if len(data) < 2+count*3 {
		return nil, fmt.Errorf("cursors truncated")
	}
	for i := range cursors {
		cursors[i] = CursorPos{
			PlayerID: data[2+i*3],
			X:        data[2+i*3+1],
			Y:        data[2+i*3+2],
		}
	}
	return cursors, nil
}