package models

// Player represents a player in the game
type Player string

const (
	PlayerX Player = "X"
	PlayerO Player = "O"
	Empty   Player = ""
)

// Board represents the 3x3 game board
type Board [9]Player

// GameState represents the current state of a game
type GameState struct {
	ID            string `json:"id"`
	Board         Board  `json:"board"`
	CurrentTurn   Player `json:"currentTurn"`
	Winner        Player `json:"winner"`
	IsOver        bool   `json:"isOver"`
	IsDraw        bool   `json:"isDraw"`
	PlayerXJoined bool   `json:"playerXJoined"`
	PlayerOJoined bool   `json:"playerOJoined"`
}

// Move represents a player's move
type Move struct {
	Position int    `json:"position"`
	Player   Player `json:"player"`
}

// NewGameState creates a new game state
func NewGameState(id string) *GameState {
	return &GameState{
		ID:          id,
		Board:       Board{},
		CurrentTurn: PlayerX,
		Winner:      Empty,
		IsOver:      false,
		IsDraw:      false,
	}
}
