package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DayActivity holds all activity data for a single day.
type DayActivity struct {
	// hostname -> user -> appName -> AppTime
	Hosts map[string]map[string]map[string]*AppTime `json:"hosts"`
}

// UserActivity is a flattened view for the UI (aggregated across hosts).
type UserActivity struct {
	Apps           map[string]*AppTime
	Total          int // only categorized apps (excludes "Sonstiges")
	ByCategory     map[string][]*AppTime
	CategoryTotals map[string]int
}

// ActivityStore manages per-day activity files on disk.
type ActivityStore struct {
	dir string
	mu  sync.Mutex
}

// NewActivityStore creates a store backed by the given directory.
func NewActivityStore(dir string) *ActivityStore {
	os.MkdirAll(dir, 0755)
	return &ActivityStore{dir: dir}
}

// Record merges an incoming report into today's activity file.
func (s *ActivityStore) Record(report Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	date := time.Now().Format("2006-01-02")
	day, err := s.loadDay(date)
	if err != nil {
		return err
	}

	if day.Hosts == nil {
		day.Hosts = make(map[string]map[string]map[string]*AppTime)
	}

	hostname := report.Hostname
	if day.Hosts[hostname] == nil {
		day.Hosts[hostname] = make(map[string]map[string]*AppTime)
	}

	for username, apps := range report.Users {
		if day.Hosts[hostname][username] == nil {
			day.Hosts[hostname][username] = make(map[string]*AppTime)
		}
		for _, app := range apps {
			existing := day.Hosts[hostname][username][app.Name]
			if existing == nil {
				existing = &AppTime{
					Name: app.Name,
				}
				day.Hosts[hostname][username][app.Name] = existing
			}
			existing.Seconds += app.Seconds
			if app.Category != nil {
				existing.Category = app.Category
			}
		}
	}

	return s.saveDay(date, day)
}

// GetDay returns the activity data for the given date, aggregated across all hosts.
func (s *ActivityStore) GetDay(date string) (map[string]*UserActivity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	day, err := s.loadDay(date)
	if err != nil {
		return nil, err
	}

	// Aggregate across all hosts
	result := make(map[string]*UserActivity)
	for _, users := range day.Hosts {
		for username, apps := range users {
			if result[username] == nil {
				result[username] = &UserActivity{
					Apps:       make(map[string]*AppTime),
					ByCategory: make(map[string][]*AppTime),
				}
			}
			ua := result[username]
			for _, app := range apps {
				existing := ua.Apps[app.Name]
				if existing == nil {
					existing = &AppTime{
						Name:     app.Name,
						Category: app.Category,
					}
					ua.Apps[app.Name] = existing
				}
				existing.Seconds += app.Seconds
			}
		}
	}

	// Build category groups and totals
	for _, ua := range result {
		ua.ByCategory = make(map[string][]*AppTime)
		ua.CategoryTotals = make(map[string]int)
		ua.Total = 0
		for _, app := range ua.Apps {
			cat := "Sonstiges"
			if app.Category != nil {
				cat = *app.Category
			}
			ua.ByCategory[cat] = append(ua.ByCategory[cat], app)
			ua.CategoryTotals[cat] += app.Seconds
			// Only count categorized apps in the total
			if cat != "Sonstiges" {
				ua.Total += app.Seconds
			}
		}
		// Sort apps within each category by seconds descending
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
	entries, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return nil, err
	}

	var dates []string
	for _, e := range entries {
		base := filepath.Base(e)
		date := strings.TrimSuffix(base, ".json")
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates, nil
}

func (s *ActivityStore) loadDay(date string) (DayActivity, error) {
	path := filepath.Join(s.dir, date+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DayActivity{}, nil
	}
	if err != nil {
		return DayActivity{}, err
	}

	var day DayActivity
	if err := json.Unmarshal(data, &day); err != nil {
		return DayActivity{}, err
	}
	return day, nil
}

func (s *ActivityStore) saveDay(date string, day DayActivity) error {
	data, err := json.MarshalIndent(day, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.dir, date+".json")
	tmp, err := os.CreateTemp(s.dir, "activity-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
