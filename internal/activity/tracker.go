package activity

import (
	"os"
	"sync"
	"time"
)

type appEntry struct {
	Category *string
	Icon     *string
	Seconds  int
}

// Tracker accumulates per-app active time on the daemon side.
type Tracker struct {
	interval time.Duration
	desktop  *DesktopLookup
	hostname string

	mu         sync.Mutex
	accum      map[string]map[string]*appEntry // user -> appName -> entry
	sessions   map[string]string               // user -> "online"/"idle"
	screenTime map[string]int                  // user -> active seconds
}

// NewTracker creates a Tracker that accumulates time in increments of interval.
func NewTracker(interval time.Duration, usernames []string) *Tracker {
	hostname, _ := os.Hostname()
	return &Tracker{
		interval: interval,
		desktop:  NewDesktopLookup(usernames),
		hostname: hostname,
		accum:      make(map[string]map[string]*appEntry),
		sessions:   make(map[string]string),
		screenTime: make(map[string]int),
	}
}

// Tick scans processes for each user and accumulates active time.
func (t *Tracker) Tick(users []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	seconds := int(t.interval.Seconds())

	for _, username := range users {
		if !IsSessionActive(username) {
			t.sessions[username] = "idle"
			continue
		}
		t.sessions[username] = "online"
		t.screenTime[username] += seconds

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

			var icon *string

			if info, ok := t.desktop.Lookup(exeBasename); ok {
				name = info.Name
				category = info.Category
				icon = info.Icon
			} else if info, ok := t.desktop.LookupSteamGame(exeBasename); ok {
				name = info.Name
				category = info.Category
				icon = info.Icon
			} else if iconURI := t.desktop.resolveIcon(exeBasename); iconURI != "" {
				// No desktop match, but try to find an icon by process name
				icon = &iconURI
			}

			entry := t.accum[username][name]
			if entry == nil {
				entry = &appEntry{Category: category, Icon: icon}
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

	if len(t.accum) == 0 && len(t.sessions) == 0 {
		return nil
	}

	report := &Report{
		Hostname:   t.hostname,
		Users:      make(map[string][]AppTime),
		Sessions:   t.sessions,
		ScreenTime: t.screenTime,
	}

	for username, apps := range t.accum {
		for name, entry := range apps {
			report.Users[username] = append(report.Users[username], AppTime{
				Name:     name,
				Category: entry.Category,
				Icon:     entry.Icon,
				Seconds:  entry.Seconds,
			})
		}
	}

	t.accum = make(map[string]map[string]*appEntry)
	t.sessions = make(map[string]string)
	t.screenTime = make(map[string]int)
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
				entry = &appEntry{Category: app.Category, Icon: app.Icon}
				t.accum[username][app.Name] = entry
			}
			entry.Seconds += app.Seconds
		}
	}
}
