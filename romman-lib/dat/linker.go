package dat

import (
	"database/sql"
	"fmt"
)

// LinkClones resolves parent/clone relationships for a specific system.
// It updates the parent_id column for releases that have a clone_of value matching another release's name in the same system.
// Returns the number of releases updated.
func LinkClones(db *sql.DB, systemID int64) (int64, error) {
	query := `
		UPDATE releases
		SET parent_id = (
			SELECT p.id 
			FROM releases p 
			WHERE p.system_id = releases.system_id 
			AND p.name = releases.clone_of
		)
		WHERE system_id = ? 
		AND clone_of IS NOT NULL 
		AND clone_of != '' 
		AND parent_id IS NULL
		AND EXISTS (
			SELECT 1
			FROM releases p 
			WHERE p.system_id = releases.system_id 
			AND p.name = releases.clone_of
		)
	`

	res, err := db.Exec(query, systemID)
	if err != nil {
		return 0, fmt.Errorf("failed to link clones: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}
