package activity

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// DayActivity is the legacy JSON storage format for migration.
type DayActivity struct {
	Hosts      map[string]map[string]map[string]*AppTime `json:"hosts"`
	ScreenTime map[string]map[string]int                  `json:"screenTime,omitempty"`
}

// MigrateFromJSON migrates existing JSON activity files to the SQLite database.
// Skips migration if the database already has data.
func MigrateFromJSON(jsonDir string, db *sql.DB) error {
	// Check if DB already has data
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_activity`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already migrated
	}

	// Find JSON files
	files, err := filepath.Glob(filepath.Join(jsonDir, "*.json"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	log.Printf("migrating %d JSON activity files to SQLite...", len(files))

	for _, path := range files {
		date := strings.TrimSuffix(filepath.Base(path), ".json")

		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("skip %s: %v", path, err)
			continue
		}

		var day DayActivity
		if err := json.Unmarshal(data, &day); err != nil {
			log.Printf("skip %s: %v", path, err)
			continue
		}

		if err := migrateDay(db, date, day); err != nil {
			log.Printf("error migrating %s: %v", path, err)
			continue
		}
	}

	log.Printf("migration complete")
	return nil
}

func migrateDay(db *sql.DB, date string, day DayActivity) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	appStmt, err := tx.Prepare(`INSERT INTO app_activity (date, hostname, user, app_name, category, seconds)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, hostname, user, app_name) DO UPDATE SET
			seconds = seconds + excluded.seconds,
			category = COALESCE(excluded.category, category)`)
	if err != nil {
		return err
	}
	defer appStmt.Close()

	iconStmt, err := tx.Prepare(`INSERT INTO icons (app_name, data_uri) VALUES (?, ?)
		ON CONFLICT(app_name) DO UPDATE SET data_uri = excluded.data_uri`)
	if err != nil {
		return err
	}
	defer iconStmt.Close()

	stStmt, err := tx.Prepare(`INSERT INTO screen_time (date, hostname, user, seconds)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(date, hostname, user) DO UPDATE SET seconds = seconds + excluded.seconds`)
	if err != nil {
		return err
	}
	defer stStmt.Close()

	for hostname, users := range day.Hosts {
		for username, apps := range users {
			for _, app := range apps {
				if _, err := appStmt.Exec(date, hostname, username, app.Name, app.Category, app.Seconds); err != nil {
					return err
				}
				if app.Icon != nil {
					if _, err := iconStmt.Exec(app.Name, *app.Icon); err != nil {
						return err
					}
				}
			}
		}
	}

	for hostname, users := range day.ScreenTime {
		for username, seconds := range users {
			if _, err := stStmt.Exec(date, hostname, username, seconds); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
