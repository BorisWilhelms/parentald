package activity

import (
	"os"
	"sync"
	"time"
)

type appEntry struct {
	Category *string
	Seconds  int
}

// Tracker accumulates per-app active time on the daemon side.
type Tracker struct {
	interval time.Duration
	desktop  *DesktopLookup
	hostname string

	mu    sync.Mutex
	accum map[string]map[string]*appEntry // user -> appName -> entry
}

// NewTracker creates a Tracker that accumulates time in increments of interval.
func NewTracker(interval time.Duration, usernames []string) *Tracker {
	hostname, _ := os.Hostname()
	return &Tracker{
		interval: interval,
		desktop:  NewDesktopLookup(usernames),
		hostname: hostname,
		accum:    make(map[string]map[string]*appEntry),
	}
}

// Tick scans processes for each user and accumulates active time.
func (t *Tracker) Tick(users []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	seconds := int(t.interval.Seconds())

	for _, username := range users {
		if !IsSessionActive(username) {
			continue
		}

		processes, err := ScanUserProcesses(username)
		if err != nil {
			continue
		}

		if t.accum[username] == nil {
			t.accum[username] = make(map[string]*appEntry)
		}

		for _, exeBasename := range processes {
			name := exeBasename
			var category *string

			if info, ok := t.desktop.Lookup(exeBasename); ok {
				name = info.Name
				category = info.Category
			}

			entry := t.accum[username][name]
			if entry == nil {
				entry = &appEntry{Category: category}
				t.accum[username][name] = entry
			}
			entry.Seconds += seconds
		}
	}
}

// Flush returns the accumulated report and resets the accumulator.
func (t *Tracker) Flush() *Report {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.accum) == 0 {
		return nil
	}

	report := &Report{
		Hostname: t.hostname,
		Users:    make(map[string][]AppTime),
	}

	for username, apps := range t.accum {
		for name, entry := range apps {
			report.Users[username] = append(report.Users[username], AppTime{
				Name:     name,
				Category: entry.Category,
				Seconds:  entry.Seconds,
			})
		}
	}

	t.accum = make(map[string]map[string]*appEntry)
	return report
}

// Merge adds a report back into the accumulator (used for retry on failed POST).
func (t *Tracker) Merge(report *Report) {
	if report == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for username, apps := range report.Users {
		if t.accum[username] == nil {
			t.accum[username] = make(map[string]*appEntry)
		}
		for _, app := range apps {
			entry := t.accum[username][app.Name]
			if entry == nil {
				entry = &appEntry{Category: app.Category}
				t.accum[username][app.Name] = entry
			}
			entry.Seconds += app.Seconds
		}
	}
}
