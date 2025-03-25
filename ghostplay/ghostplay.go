package ghostplay

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// Common errors that can be checked
var (
	ErrDatabaseConnection = errors.New("database connection error")
	ErrPlayerNotFound     = errors.New("player not found")
	ErrInvalidData        = errors.New("invalid player data")
)

// PlayerState stores the data for each user.
// The ExtraData field will be marshaled into a JSON string.
// Make sure that the ExtraData field is a struct of the data you wish to add.
// The flags field is a map of indicators for system-level statuses
// (e.g., "tutorial_completed": true, "is_premium": false)
type PlayerState[T any] struct {
	ExtraData   T               `json:"extra_data"`
	XP          uint64          `json:"xp"`
	Level       uint32          `json:"level"`
	LastUpdated time.Time       `json:"last_updated"`
	ID          uuid.UUID       `json:"id"`
	UserName    string          `json:"user_name"`
	Phrase      string          `json:"phrase"`
	Flags       map[string]bool `json:"flags"`
}

// InitPlayerStateTable creates the player state table if it doesn't exist
func InitPlayerStateTable(db *sql.DB, dbTableName string) error {
	if db == nil {
		return fmt.Errorf("%w: nil database connection", ErrDatabaseConnection)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			phrase VARCHAR(255) UNIQUE NOT NULL,
			user_name VARCHAR(255) NOT NULL,
			level INT4 NOT NULL DEFAULT 1,
			xp INT8 NOT NULL DEFAULT 0,
			last_updated TIMESTAMPTZ NOT NULL DEFAULT now(),
			flags JSONB DEFAULT '{}',
			extra_data JSONB DEFAULT '{}'
		)
	`, dbTableName)

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create player state table: %w", err)
	}
	return nil
}

// InitPlayer creates a new player in the database
func InitPlayer(db *sql.DB, id uuid.UUID, username, phrase, dbTableName string) error {
	if db == nil {
		return fmt.Errorf("%w: nil database connection", ErrDatabaseConnection)
	}

	if id == uuid.Nil {
		return fmt.Errorf("%w: player ID cannot be nil", ErrInvalidData)
	}

	if username == "" || phrase == "" {
		return fmt.Errorf("%w: username and phrase cannot be empty", ErrInvalidData)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_name, phrase)
		VALUES ($1, $2, $3)
		`, dbTableName)

	_, err := db.Exec(query, id, username, phrase)
	if err != nil {
		return fmt.Errorf("failed to create player: %w", err)
	}

	return nil
}

// GetUserStateByID takes the UUID for a player and returns a player state struct.
func GetUserStateByID[T any](db *sql.DB, dbTableName string, id uuid.UUID) (*PlayerState[T], error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil database connection", ErrDatabaseConnection)
	}

	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: player ID cannot be nil", ErrInvalidData)
	}

	var state PlayerState[T]
	state.Flags = make(map[string]bool)

	query := fmt.Sprintf(`
		SELECT id, user_name, phrase, level, xp, last_updated, flags, extra_data
		FROM %s
		WHERE id = $1
		`, dbTableName)

	var flagsJSON, extraJSON []byte
	err := db.QueryRow(query, id).Scan(
		&state.ID,
		&state.UserName,
		&state.Phrase,
		&state.Level,
		&state.XP,
		&state.LastUpdated,
		&flagsJSON,
		&extraJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrPlayerNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query player data: %w", err)
	}

	// Unmarshal the JSON fields
	if err := json.Unmarshal(flagsJSON, &state.Flags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flags: %w", err)
	}

	if err := json.Unmarshal(extraJSON, &state.ExtraData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extra data: %w", err)
	}

	return &state, nil
}

// GetUserStateByPhrase takes in the database table and user passphrase and
// returns a PlayerState sturct.
func GetUserStateByPhrase[T any](db *sql.DB, dbTableName, phrase string) (*PlayerState[T], error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil database connection", ErrDatabaseConnection)
	}

	if phrase == "" {
		return nil, fmt.Errorf("%w: phrase cannot be empty", ErrInvalidData)
	}

	var state PlayerState[T]
	state.Flags = make(map[string]bool)

	query := fmt.Sprintf(`
		SELECT id, user_name, phrase, level, xp, last_updated, flags, extra_data
		FROM %s
		WHERE phrase = $1
		`, dbTableName)

	var flagsJSON, extraJSON []byte
	err := db.QueryRow(query, phrase).Scan(
		&state.ID,
		&state.UserName,
		&state.Phrase,
		&state.Level,
		&state.XP,
		&state.LastUpdated,
		&flagsJSON,
		&extraJSON,
	)

	if err == sql.ErrNoRows {
		return nil, ErrPlayerNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query player data by phrase: %w", err)
	}

	// Unmarshal the JSON fields
	if err := json.Unmarshal(flagsJSON, &state.Flags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flags: %w", err)
	}

	if err := json.Unmarshal(extraJSON, &state.ExtraData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extra data: %w", err)
	}

	return &state, nil
}

// Save takes the existing player and updates the DB with the new player information.
// If the player does not exist; this function will initiate a DB entry with the provided
// data and return.
func (p *PlayerState[T]) Save(db *sql.DB, dbTableName string, xpIncrease uint64) error {
	if db == nil {
		return fmt.Errorf("%w: nil database connection", ErrDatabaseConnection)
	}

	if p.ID == uuid.Nil {
		// Generate a new ID if needed
		p.ID = uuid.New()
	}

	if p.UserName == "" || p.Phrase == "" {
		return fmt.Errorf("%w: username and phrase cannot be empty", ErrInvalidData)
	}

	player, err := GetUserStateByID[T](db, dbTableName, p.ID)
	if err != nil && !errors.Is(err, ErrPlayerNotFound) {
		return fmt.Errorf("failed to fetch player state: %w", err)
	}

	if player == nil || errors.Is(err, ErrPlayerNotFound) {
		log.Printf("Creating new player: %s\n", p.UserName)

		// Initialize any nil fields
		if p.Flags == nil {
			p.Flags = make(map[string]bool)
		}

		// Set default values for new player
		p.Level = 1
		p.XP = xpIncrease
		p.LastUpdated = time.Now()

		// Create new player
		err = InitPlayer(db, p.ID, p.UserName, p.Phrase, dbTableName)
		if err != nil {
			return fmt.Errorf("failed to initialize player: %w", err)
		}

		// If we just initialized with base values, we need to update with the complete state
		extraData, err := json.Marshal(p.ExtraData)
		if err != nil {
			return fmt.Errorf("failed to marshal extra data: %w", err)
		}

		flags, err := json.Marshal(p.Flags)
		if err != nil {
			return fmt.Errorf("failed to marshal flags: %w", err)
		}

		query := fmt.Sprintf(`
		UPDATE %s
		SET level = $1,
			xp = $2,
			extra_data = $3,
			flags = $4,
			last_updated = $5
		WHERE id = $6
			`, dbTableName)

		_, err = db.Exec(query,
			p.Level,
			p.XP,
			extraData,
			flags,
			p.LastUpdated,
			p.ID,
		)

		if err != nil {
			return fmt.Errorf("failed to update new player data: %w", err)
		}

		return nil
	}

	// Update existing player
	p.XP = player.XP + xpIncrease
	p.LastUpdated = time.Now()

	// Calculate level up
	xpThreshold := (uint64(p.Level) * 200)
	if p.XP >= xpThreshold && p.Level < player.Level+1 {
		p.Level = player.Level + 1
	}

	extraData, err := json.Marshal(p.ExtraData)
	if err != nil {
		return fmt.Errorf("failed to marshal extra data: %w", err)
	}

	flags, err := json.Marshal(p.Flags)
	if err != nil {
		return fmt.Errorf("failed to marshal flags: %w", err)
	}

	query := fmt.Sprintf(`
	UPDATE %s
	SET level = $1,
		xp = $2,
		extra_data = $3,
		flags = $4,
		last_updated = $5
	WHERE id = $6
		`, dbTableName)

	_, err = db.Exec(query,
		p.Level,
		p.XP,
		extraData,
		flags,
		p.LastUpdated,
		p.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update player data: %w", err)
	}

	return nil
}

// Leader represents a player on the leaderboard
type Leader struct {
	UserName string `db:"user_name"`
	Level    uint32 `db:"level"`
	XP       uint64 `db:"xp"`
}

// GetLeaderboard fetches the top users by XP.
func GetLeaderboard(db *sql.DB, dbTableName string, limit int) ([]Leader, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil database connection", ErrDatabaseConnection)
	}

	if limit <= 0 {
		return nil, fmt.Errorf("%w: leaderboard limit must be greater than zero", ErrInvalidData)
	}

	query := fmt.Sprintf(`
		SELECT user_name, level, xp
		FROM %s
		ORDER BY xp DESC
		LIMIT $1`, dbTableName)

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}
	defer rows.Close()

	var users []Leader
	for rows.Next() {
		var user Leader
		err := rows.Scan(
			&user.UserName,
			&user.Level,
			&user.XP,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard row: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating through leaderboard rows: %w", err)
	}

	return users, nil
}
