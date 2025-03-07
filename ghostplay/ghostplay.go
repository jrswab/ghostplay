package ghostplay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
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

func initPlayerStateTable(db *sql.DB, dbTableName string) error {
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
	return err
}

func initPlayer(db *sql.DB, id uuid.UUID, username, phrase, dbTableName string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_name, phrase)
		VALUES ($1, $2, $3)
		`, dbTableName)
	_, err := db.Exec(query, id, username, phrase)

	return err
}

// GetUserStateByID takes the UUID for a player and returns a player state struct.
func GetUserStateByID[T any](db *sql.DB, dbTableName string, id uuid.UUID) (*PlayerState[T], error) {
	var state PlayerState[T]

	query := fmt.Sprintf(`
		SELECT *
		FROM %s
		WHERE id = $1
		`, dbTableName)

	err := db.QueryRow(query, id).Scan(
		&state.ID,
		&state.UserName,
		&state.Level,
		&state.XP,
		&state.ExtraData,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("ghostplay failed to query the database; %s", err)
	}

	return &state, nil
}

// GetUserStateByPhrase takes in the database table and user passphrase and
// returns a PlayerState sturct.
func GetUserStateByPhrase[T any](db *sql.DB, dbTableName, phrase string) (*PlayerState[T], error) {
	var state PlayerState[T]

	query := fmt.Sprintf(`
		SELECT *
		FROM %s
		WHERE phrase = $1
		`, dbTableName)

	err := db.QueryRow(query, dbTableName, phrase).Scan(
		&state.ID,
		&state.UserName,
		&state.Level,
		&state.XP,
		&state.ExtraData,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &state, nil
}

// Save takes the existing player and updates the DB with the new player information.
// If the player does not exist; this function will initiate a DB entry with the provided
// data and return.
func (p *PlayerState[T]) Save(db *sql.DB, dbTableName string, xpIncrease uint64) error {
	player, err := GetUserStateByID[T](db, dbTableName, p.ID)
	if err != nil {
		return fmt.Errorf("failed to update player stats; %s", err)
	}

	if player == nil {
		log.Printf("player id does not exist; creating...\n")
		// Create new UUID
		// Create new UserName
		// Create new passphrase
		// initPlayer(db, id, username, phrase, dbTableName)
		return nil
	}

	player.XP = player.XP + xpIncrease

	xpThreshold := (uint64(player.Level) * 200) + player.XP
	if player.XP%xpThreshold == 0 {
		player.Level++
	}

	extraData, err := json.Marshal(p.ExtraData)
	if err != nil {
		return fmt.Errorf("ghostplay is unable to marshal extraData; %s", err)
	}

	query := fmt.Sprintf(`
	UPDATE %s
	SET level = $1,
		xp = $2,
		data = $3
	WHERE id = $4
		`, dbTableName)

	_, err = db.Exec(query,
		dbTableName,
		player.Level,
		player.XP,
		extraData,
		player.ID,
	)

	return nil
}

type Leader struct {
	UserName string `db:"user_name"`
	Level    uint32 `db:"level"`
	XP       uint64 `db:"xp"`
}

// GetLeaderboard fetches the top users by XP.
func GetLeaderboard(db *sql.DB, dbTableName string, limit int) ([]Leader, error) {
	query := fmt.Sprintf(`
		SELECT *
		FROM %s
		ORDER BY xp DESC
		LIMIT $1`, dbTableName)

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}
