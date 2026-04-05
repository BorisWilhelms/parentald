package activity

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// UserActivity is a flattened view for the UI (aggregated across hosts).
type UserActivity struct {
	Apps           map[string]*AppTime
	ScreenTime     int
	ByCategory     map[string][]*AppTime
	CategoryTotals map[string]int
}

// UserStatus tracks the last known state of a user session.
type UserStatus struct {
	Status   string    `json:"status"`
	Hostname string    `json:"hostname"`
	LastSeen time.Time `json:"lastSeen"`
}

// DateValue holds a date-value pair for chart data.
type DateValue struct {
	Date    string `json:"date"`
	Seconds int    `json:"seconds"`
}

// AppDateValue holds per-app date-value data for chart data.
type AppDateValue struct {
	Date    string `json:"date"`
	AppName string `json:"appName"`
	Seconds int    `json:"seconds"`
}

// ActivityStore manages activity data in SQLite.
type ActivityStore struct {
	db       *sql.DB
	statusMu sync.RWMutex
	statuses map[string]*UserStatus
}

// NewActivityStore opens (or creates) the SQLite database and creates tables.
func NewActivityStore(dataDir string) *ActivityStore {
	os.MkdirAll(dataDir, 0755)
	dbPath := fmt.Sprintf("%s/parentald.db", dataDir)
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("failed to open SQLite database: %v", err)
	}

	if err := createTables(db); err != nil {
		log.Fatalf("failed to create tables: %v", err)
	}

	return &ActivityStore{
		db:       db,
		statuses: make(map[string]*UserStatus),
	}
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS icons (
			app_name TEXT PRIMARY KEY,
			data_uri TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS app_activity (
			date     TEXT NOT NULL,
			hostname TEXT NOT NULL,
			user     TEXT NOT NULL,
			app_name TEXT NOT NULL,
			category TEXT,
			seconds  INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (date, hostname, user, app_name)
		)`,
		`CREATE TABLE IF NOT EXISTS screen_time (
			date     TEXT NOT NULL,
			hostname TEXT NOT NULL,
			user     TEXT NOT NULL,
			seconds  INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (date, hostname, user)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_app_activity_user_date ON app_activity(user, date)`,
		`CREATE INDEX IF NOT EXISTS idx_screen_time_user_date ON screen_time(user, date)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

// Record inserts or updates activity data from a daemon report.
func (s *ActivityStore) Record(report Report) error {
	date := time.Now().Format("2006-01-02")
	hostname := report.Hostname

	tx, err := s.db.Begin()
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

	for username, apps := range report.Users {
		for _, app := range apps {
			var cat *string
			if app.Category != nil {
				cat = app.Category
			}
			if _, err := appStmt.Exec(date, hostname, username, app.Name, cat, app.Seconds); err != nil {
				return err
			}
			if app.Icon != nil {
				if _, err := iconStmt.Exec(app.Name, *app.Icon); err != nil {
					return err
				}
			}
		}
	}

	for username, seconds := range report.ScreenTime {
		if _, err := stStmt.Exec(date, hostname, username, seconds); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Update session statuses (in-memory)
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	now := time.Now()
	for username, status := range report.Sessions {
		s.statuses[username] = &UserStatus{
			Status:   status,
			Hostname: hostname,
			LastSeen: now,
		}
	}

	return nil
}

// GetStatuses returns the current status of all tracked users.
func (s *ActivityStore) GetStatuses() map[string]*UserStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()

	cutoff := time.Now().Add(-2 * time.Minute)
	result := make(map[string]*UserStatus, len(s.statuses))
	for user, st := range s.statuses {
		status := *st
		if status.LastSeen.Before(cutoff) {
			status.Status = "offline"
		}
		result[user] = &status
	}
	return result
}

// GetDay returns the activity data for the given date, aggregated across all hosts.
func (s *ActivityStore) GetDay(date string) (map[string]*UserActivity, error) {
	result := make(map[string]*UserActivity)

	// Query app activity
	rows, err := s.db.Query(`SELECT a.user, a.app_name, a.category, SUM(a.seconds), i.data_uri
		FROM app_activity a
		LEFT JOIN icons i ON a.app_name = i.app_name
		WHERE a.date = ?
		GROUP BY a.user, a.app_name
		ORDER BY a.user, SUM(a.seconds) DESC`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var username, appName string
		var category sql.NullString
		var seconds int
		var icon sql.NullString

		if err := rows.Scan(&username, &appName, &category, &seconds, &icon); err != nil {
			return nil, err
		}

		if result[username] == nil {
			result[username] = &UserActivity{
				Apps:       make(map[string]*AppTime),
				ByCategory: make(map[string][]*AppTime),
			}
		}

		app := &AppTime{Name: appName, Seconds: seconds}
		if category.Valid {
			app.Category = &category.String
		}
		if icon.Valid {
			app.Icon = &icon.String
		}
		result[username].Apps[appName] = app
	}

	// Query screen time
	stRows, err := s.db.Query(`SELECT user, SUM(seconds) FROM screen_time
		WHERE date = ? GROUP BY user`, date)
	if err != nil {
		return nil, err
	}
	defer stRows.Close()

	for stRows.Next() {
		var username string
		var seconds int
		if err := stRows.Scan(&username, &seconds); err != nil {
			return nil, err
		}
		if result[username] == nil {
			result[username] = &UserActivity{
				Apps:       make(map[string]*AppTime),
				ByCategory: make(map[string][]*AppTime),
			}
		}
		result[username].ScreenTime = seconds
	}

	// Build category groups
	for _, ua := range result {
		ua.ByCategory = make(map[string][]*AppTime)
		ua.CategoryTotals = make(map[string]int)
		for _, app := range ua.Apps {
			cat := "Other"
			if app.Category != nil {
				cat = *app.Category
			}
			ua.ByCategory[cat] = append(ua.ByCategory[cat], app)
			ua.CategoryTotals[cat] += app.Seconds
		}
		for _, apps := range ua.ByCategory {
			sort.Slice(apps, func(i, j int) bool {
				return apps[i].Seconds > apps[j].Seconds
			})
		}
	}

	return result, nil
}

// ListDays returns available date strings, newest first.
func (s *ActivityStore) ListDays() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT date FROM app_activity ORDER BY date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, nil
}

// GetScreenTimeRange returns daily screen time for a user within a date range.
func (s *ActivityStore) GetScreenTimeRange(user, from, to string) ([]DateValue, error) {
	rows, err := s.db.Query(`SELECT date, SUM(seconds) FROM screen_time
		WHERE user = ? AND date BETWEEN ? AND ?
		GROUP BY date ORDER BY date`, user, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DateValue
	for rows.Next() {
		var dv DateValue
		if err := rows.Scan(&dv.Date, &dv.Seconds); err != nil {
			return nil, err
		}
		result = append(result, dv)
	}
	return result, nil
}

// GetAppTimeRange returns daily per-app time for a user within a date range.
// If app is empty, returns all apps.
func (s *ActivityStore) GetAppTimeRange(user, app, from, to string) ([]AppDateValue, error) {
	var rows *sql.Rows
	var err error

	if app != "" {
		rows, err = s.db.Query(`SELECT date, app_name, SUM(seconds) FROM app_activity
			WHERE user = ? AND app_name = ? AND date BETWEEN ? AND ?
			GROUP BY date, app_name ORDER BY date`, user, app, from, to)
	} else {
		rows, err = s.db.Query(`SELECT date, app_name, SUM(seconds) FROM app_activity
			WHERE user = ? AND date BETWEEN ? AND ? AND category IS NOT NULL
			GROUP BY date, app_name ORDER BY date, SUM(seconds) DESC`, user, from, to)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AppDateValue
	for rows.Next() {
		var adv AppDateValue
		if err := rows.Scan(&adv.Date, &adv.AppName, &adv.Seconds); err != nil {
			return nil, err
		}
		result = append(result, adv)
	}
	return result, nil
}

// ListUsers returns all users that have activity data.
func (s *ActivityStore) ListUsers() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT user FROM app_activity ORDER BY user`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// ListApps returns all app names for a user that have a category (not "Other").
func (s *ActivityStore) ListApps(user string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT app_name FROM app_activity
		WHERE user = ? AND category IS NOT NULL ORDER BY app_name`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []string
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, nil
}

// DB returns the underlying database connection (for migration).
func (s *ActivityStore) DB() *sql.DB {
	return s.db
}
