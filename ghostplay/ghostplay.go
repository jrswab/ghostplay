package ghostplay

import (
	"time"
)

// Player represents any type that can be identified
type Player[ID comparable] interface {
	GetID() ID
}

// Progress represents game-related statistics and achievements
type Progress interface {
	GetLevel() uint16
	GetExperience() uint32
	GetBadges() []Badge
	GetAchievements() []Achievement
}

// GameState combines player identity with their progress
type GameState[ID comparable] interface {
	Player[ID]
	Progress
}

// Badge represents a milestone or accomplishment
type Badge struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UnlockedAt  time.Time `json:"unlocked_at"`
}

// Achievement represents a specific goal or milestone
type Achievement struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Progress    uint32    `json:"progress"`
	Target      uint32    `json:"target"`
	UnlockedAt  time.Time `json:"unlocked_at,omitempty"`
}

// GameEngine manages game state and progression
type GameEngine[ID comparable, State GameState[ID]] struct {
	storage Storage[ID, State]
}

// Storage defines the interface for persisting game state
type Storage[ID comparable, State GameState[ID]] interface {
	Get(id ID) (State, error)
	Save(state State) error
}

// NewGameEngine creates a new game engine instance
func NewGameEngine[ID comparable, State GameState[ID]](storage Storage[ID, State]) *GameEngine[ID, State] {
	return &GameEngine[ID, State]{
		storage: storage,
	}
}

// GetPlayerState retrieves the current state for a player
func (e *GameEngine[ID, State]) GetPlayerState(id ID) (State, error) {
	return e.storage.Get(id)
}

// UpdateProgress updates a player's progress and checks for new achievements
func (e *GameEngine[ID, State]) UpdateProgress(state State, points uint32) error {
	// TODO: Implement progress update logic
	return e.storage.Save(state)
}
