// internal/client/input.go
// Single responsibility: translate tcell key events → protocol directions.
// Knows about keyboards. Knows nothing about rendering or networking.

package client

import (
	"github.com/gdamore/tcell/v2"
	"github.com/siyad01/multiplayer-terminal-games/internal/protocol"
)

// KeyEvent represents a parsed key event from the terminal.
type KeyEvent struct {
	Dir  protocol.Direction // non-zero if arrow key
	Quit bool               // true if Q or Ctrl+C
}

// ParseKey translates a tcell EventKey into a KeyEvent.
// Returns a zero KeyEvent if the key is not relevant to the game.
func ParseKey(ev *tcell.EventKey) KeyEvent {
	switch ev.Key() {
	case tcell.KeyUp:
		return KeyEvent{Dir: protocol.DirUp}
	case tcell.KeyDown:
		return KeyEvent{Dir: protocol.DirDown}
	case tcell.KeyLeft:
		return KeyEvent{Dir: protocol.DirLeft}
	case tcell.KeyRight:
		return KeyEvent{Dir: protocol.DirRight}
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return KeyEvent{Quit: true}
	}

	// Check rune for Q/q
	if ev.Key() == tcell.KeyRune {
		switch ev.Rune() {
		case 'q', 'Q':
			return KeyEvent{Quit: true}
		// WASD support — maps to directions
		case 'w', 'W':
			return KeyEvent{Dir: protocol.DirUp}
		case 's', 'S':
			return KeyEvent{Dir: protocol.DirDown}
		case 'a', 'A':
			return KeyEvent{Dir: protocol.DirLeft}
		case 'd', 'D':
			return KeyEvent{Dir: protocol.DirRight}
		}
	}

	return KeyEvent{} // not a game key
}