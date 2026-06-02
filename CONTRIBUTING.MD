# Contributing to multiplayer-terminal-games

## Getting started

```bash
git clone https://github.com/siyad01/multiplayer-terminal-games
cd multiplayer-terminal-games
go build ./...
go test ./...
```

All 29 tests must pass before and after your change.

## Project structure

| Package | Responsibility | Change if you are... |
|---------|---------------|----------------------|
| `internal/protocol` | Wire format | Adding a new message type |
| `internal/snake` | Snake game logic | Fixing game rules or levels |
| `internal/minesweeper` | Mine logic + renderer | Fixing mine logic or display |
| `internal/server` | WebSocket hub | Changing connection handling |
| `internal/client` | Snake renderer + input | Changing snake display |
| `cmd/server` | Snake server entry point | Changing server startup |
| `cmd/mineserver` | Mine server entry point | Changing mine server startup |
| `cmd/client` | Unified client | Changing menus or game routing |

## Rules

- `go test ./...` — all tests pass
- `go build ./...` — zero warnings
- One responsibility per file
- New message types get encode + decode + round-trip test
- New game logic gets a correctness test — test what should be denied, not just what works

## What is welcome

- Bug fixes with a regression test
- New Snake levels or level mechanics
- Minesweeper difficulty presets
- Better terminal rendering for different font sizes
- Performance improvements

## What needs a discussion first (open an issue)

- New games (significant scope)
- Protocol changes that break existing clients
- New dependencies

## Coding style

- No `unwrap()` equivalent — handle all errors explicitly
- No silent failures — log or return errors, never swallow them
- Channels for cross-goroutine communication, not shared state
- `go fmt` before committing