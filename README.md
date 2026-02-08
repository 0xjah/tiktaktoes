# Tic Tac Toe

Real time multiplayer tiktaktoes game.

## Run

```bash
go run ./cmd/server
```

Open http://localhost:8080

## Play

1. Click **[new]** to create a game
2. Share the link with a friend
3. Friend opens link and selects **O**
4. Take turns clicking cells

## Structure

```
cmd/server/         - Entry point
internal/models/    - Data models
internal/game/      - Game logic
internal/api/       - HTTP & WebSocket handlers
web/                - Frontend
```
