<div align="center">

# 🎮 multiplayer-terminal-games

**Real-time multiplayer games that run entirely in your terminal**

*Snake and Minesweeper — playable over WebSockets from any terminal, anywhere on your network*

[![Build](https://github.com/siyad01/multiplayer-terminal-games/actions/workflows/build.yml/badge.svg)](https://github.com/siyad01/multiplayer-terminal-games/actions/workflows/build.yml)
[![Tests](https://img.shields.io/badge/Tests-29%20passing-22c55e)](#)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows%20%7C%20WSL2-0ea5e9)](#install)

</div>

---

## Why this exists

Every multiplayer game framework assumes you have a browser, an app store, or a GPU.
This one assumes you have a terminal.

**multiplayer-terminal-games** is a WebSocket game server with two fully playable games —
Snake and Minesweeper — that render entirely in the terminal using raw mode.
No browser. No installation beyond a single binary. Works over SSH.

---

## Games

### Snake

Classic Snake with 10 levels, fruits, and poison.

| Mode | Description |
|------|-------------|
| Single Player | 10 levels, eat fruit to advance, avoid poison and walls |
| Last Standing | Multiplayer — last snake alive wins |
| Score Race | Multiplayer — most food eaten in the time limit wins |

- Levels 1-10: food count increases, poison added from level 2
- Different fruit emoji per level: 🍎 🍊 🍋 🍇 🍓 🍑 🍍 🥝 🍒 🫐
- Live scoreboard + timer (Score Race mode only)
- Host controls game duration with `+` / `-`

### Minesweeper

Classic Minesweeper with three difficulty levels and three play modes.

| Mode | Description |
|------|-------------|
| Solo | Classic single-player, clear the board without hitting a mine |
| Cooperative | Shared board — players work together to clear it |
| Competitive | Shared board — most cells revealed wins |

| Difficulty | Board | Mines |
|------------|-------|-------|
| Easy | 9×9 | 10 |
| Medium | 16×16 | 40 |
| Hard | 30×16 | 99 |

- Safe first click — first reveal never hits a mine
- Flood-fill reveal for empty cells
- Flag suspected mines with `F`
- Multiplayer cursors shown in real time

---

## Install

```bash
git clone https://github.com/siyad01/multiplayer-terminal-games
cd multiplayer-terminal-games
go build -o bin/snake-server    ./cmd/server
go build -o bin/mine-server     ./cmd/mineserver
go build -o bin/game-client     ./cmd/client
```

Or install directly:

```bash
go install github.com/siyad01/multiplayer-terminal-games/cmd/server@latest
go install github.com/siyad01/multiplayer-terminal-games/cmd/mineserver@latest
go install github.com/siyad01/multiplayer-terminal-games/cmd/client@latest
```

**Requirements:** Go 1.21+ — no other runtime, no external services.

---

## Quick start

```bash
# Terminal 1 — Snake server
./bin/snake-server

# Terminal 2 — Minesweeper server
./bin/mine-server

# Terminal 3+ — any number of players
./bin/game-client -name yourname

# Play on another machine on the same network
./bin/game-client -name yourname -server 192.168.x.x:8080
```

---

## Controls

### Snake

| Key | Action |
|-----|--------|
| `↑ ↓ ← →` or `WASD` | Move snake |
| `SPACE` / `ENTER` | Start game (host) |
| `+` / `-` | Adjust race timer (host, Score Race mode) |
| `M` | Cycle multiplayer mode (host) |
| `R` | Play again after game over |
| `Q` | Quit |

### Minesweeper

| Key | Action |
|-----|--------|
| `↑ ↓ ← →` or `WASD` | Move cursor |
| `ENTER` | Reveal cell |
| `F` | Flag / unflag mine |
| `TAB` | Cycle difficulty (menu) |
| `R` | Play again after game over |
| `Q` | Quit |

---

## Architecture

```
multiplayer-terminal-games/
├── cmd/
│   ├── server/       # Snake WebSocket server  (:8080)
│   ├── mineserver/   # Minesweeper server       (:8081)
│   └── client/       # Unified terminal client
│
└── internal/
    ├── protocol/     # Binary wire format — shared by client + server
    ├── snake/        # Snake game logic + loop (pure, no networking)
    ├── minesweeper/  # Minesweeper logic + loop + renderer
    ├── server/       # Hub: WebSocket connection manager
    └── client/       # Snake renderer + input handler
```
[Client terminal]
│
│  WebSocket (binary protocol)
▼
[Hub] ──► [Game Loop goroutine]  ←── ticker / events
◄── broadcasts every tick/action

**Key design decisions:**

- **Single-owner concurrency** — game state is owned by exactly one goroutine. All other goroutines communicate through typed channels. No mutexes on game state.
- **Binary protocol** — custom binary encoding, not JSON. Every message has a 1-byte type header. Game state for a 30×20 board = ~65 bytes per frame.
- **Event-driven Minesweeper** — no ticker. The loop only wakes on player actions.
- **Safe first click** — mines placed after first reveal, guaranteed safe 3×3 zone.

---

## How the protocol works

Every message on the wire starts with a 1-byte type:

Client → Server          Server → Client
──────────────           ───────────────
0x01  Join               0x10  GameState (board snapshot)
0x02  Input (direction)  0x11  PlayerJoined
0x03  Leave              0x12  PlayerLeft
0x04  StartGame          0x13  GameOver
0x05  SetTime            0x14  Welcome (assigned player ID)
0x06  PlayAgain          0x15  LobbyState
0x07  NextLevel          0x16  TimerUpdate
0x08  SetMode            0x17  LevelUp
0x09  SetMineMode        0x18  LevelComplete
0x20  MineReveal         0x19  GameComplete
0x21  MineFlag           0x1A  GameMode
0x22  MineCursor         0x30  MineState
0x31  MineUpdate (partial)
0x32  MineWin
0x33  MineLose
0x34  MineCursors

---

## Test coverage
29 tests across 4 packages — 0 failures
internal/protocol     12 tests  wire format round-trips
internal/snake        11 tests  game logic + food eating (all 4 directions)
internal/minesweeper   9 tests  mine placement, flood fill, flag toggle
internal/server        4 tests  hub ID recycling, broadcast, max players

Run tests:

```bash
go test ./...
```

---

## Comparison

| Feature | This project | typical game tutorial |
|---------|-------------|----------------------|
| Real multiplayer | ✅ WebSocket | ❌ single player only |
| Terminal rendering | ✅ tcell raw mode | ❌ browser only |
| Two games | ✅ Snake + Minesweeper | ❌ one game |
| Binary protocol | ✅ custom, documented | ❌ JSON |
| Tests | ✅ 29 passing | ❌ none |
| Single binary | ✅ no runtime | ❌ node_modules |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

## License

Apache 2.0 — see [LICENSE](LICENSE)

---

<div align="center">

Built with Go · WebSockets · tcell · zero browser dependencies

**If you've ever wanted to play Snake over SSH, this is it. Give it a ⭐**

</div>