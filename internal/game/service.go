package game

import (
	"errors"
	"sync"
	"tiktaktoes/internal/models"

	"github.com/google/uuid"
)

var (
	ErrInvalidMove   = errors.New("invalid move")
	ErrNotYourTurn   = errors.New("not your turn")
	ErrGameOver      = errors.New("game is over")
	ErrPositionTaken = errors.New("position already taken")
	ErrGameFull      = errors.New("game is full, already has two players")
	ErrSlotTaken     = errors.New("that player slot is already taken")
	ErrInvalidPlayer = errors.New("invalid player, must be X or O")
)

// winConditions defines all possible winning combinations
var winConditions = [][]int{
	{0, 1, 2}, // top row
	{3, 4, 5}, // middle row
	{6, 7, 8}, // bottom row
	{0, 3, 6}, // left column
	{1, 4, 7}, // middle column
	{2, 5, 8}, // right column
	{0, 4, 8}, // diagonal
	{2, 4, 6}, // anti-diagonal
}

// Service handles game logic
type Service struct {
	games map[string]*models.GameState
	mu    sync.RWMutex
}

// NewService creates a new game service
func NewService() *Service {
	return &Service{
		games: make(map[string]*models.GameState),
	}
}

// CreateGame creates a new game and returns its state.
// The creator automatically joins as the given player.
func (s *Service) CreateGame(creator models.Player) *models.GameState {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()[:8]
	game := models.NewGameState(id)

	if creator == models.PlayerX {
		game.PlayerXJoined = true
	} else if creator == models.PlayerO {
		game.PlayerOJoined = true
	}

	s.games[id] = game
	return game
}

// JoinGame attempts to join a game as the given player.
// Returns an error if the game is full or the slot is already taken.
func (s *Service) JoinGame(gameID string, player models.Player) (*models.GameState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, exists := s.games[gameID]
	if !exists {
		return nil, errors.New("game not found")
	}

	if player != models.PlayerX && player != models.PlayerO {
		return nil, ErrInvalidPlayer
	}

	// Check if the requested slot is already taken
	if player == models.PlayerX && game.PlayerXJoined {
		return nil, ErrSlotTaken
	}
	if player == models.PlayerO && game.PlayerOJoined {
		return nil, ErrSlotTaken
	}

	// Check if game already has 2 players
	if game.PlayerXJoined && game.PlayerOJoined {
		return nil, ErrGameFull
	}

	// Join
	if player == models.PlayerX {
		game.PlayerXJoined = true
	} else {
		game.PlayerOJoined = true
	}

	return game, nil
}

// GetGame retrieves a game by ID
func (s *Service) GetGame(id string) (*models.GameState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	game, exists := s.games[id]
	return game, exists
}

// MakeMove processes a move and returns updated game state
func (s *Service) MakeMove(gameID string, move models.Move) (*models.GameState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, exists := s.games[gameID]
	if !exists {
		return nil, errors.New("game not found")
	}

	if game.IsOver {
		return nil, ErrGameOver
	}

	if move.Position < 0 || move.Position > 8 {
		return nil, ErrInvalidMove
	}

	if game.Board[move.Position] != models.Empty {
		return nil, ErrPositionTaken
	}

	if move.Player != game.CurrentTurn {
		return nil, ErrNotYourTurn
	}

	// Make the move
	game.Board[move.Position] = move.Player

	// Check for winner
	if winner := s.checkWinner(game.Board); winner != models.Empty {
		game.Winner = winner
		game.IsOver = true
	} else if s.isBoardFull(game.Board) {
		game.IsDraw = true
		game.IsOver = true
	} else {
		// Switch turns
		if game.CurrentTurn == models.PlayerX {
			game.CurrentTurn = models.PlayerO
		} else {
			game.CurrentTurn = models.PlayerX
		}
	}

	return game, nil
}

// ResetGame resets an existing game
func (s *Service) ResetGame(gameID string) (*models.GameState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.games[gameID]
	if !exists {
		return nil, errors.New("game not found")
	}

	game := models.NewGameState(gameID)
	s.games[gameID] = game
	return game, nil
}

// checkWinner checks if there's a winner
func (s *Service) checkWinner(board models.Board) models.Player {
	for _, condition := range winConditions {
		a, b, c := condition[0], condition[1], condition[2]
		if board[a] != models.Empty && board[a] == board[b] && board[b] == board[c] {
			return board[a]
		}
	}
	return models.Empty
}

// isBoardFull checks if the board is full
func (s *Service) isBoardFull(board models.Board) bool {
	for _, cell := range board {
		if cell == models.Empty {
			return false
		}
	}
	return true
}
