package activity

import (
	"path/filepath"
	"testing"
)

func strPtr(s string) *string { return &s }

func newTestStore(t *testing.T) *ActivityStore {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	return NewActivityStore(dir)
}

func TestActivityStore_RecordAndGet(t *testing.T) {
	store := newTestStore(t)

	report := Report{
		Hostname: "pc1",
		Users: map[string][]AppTime{
			"kind1": {
				{Name: "Firefox", Category: strPtr("Network"), Seconds: 60},
				{Name: "some-daemon", Category: nil, Seconds: 120},
			},
		},
		ScreenTime: map[string]int{"kind1": 180},
	}

	if err := store.Record(report); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Record again — should merge
	report2 := Report{
		Hostname: "pc1",
		Users: map[string][]AppTime{
			"kind1": {
				{Name: "Firefox", Category: strPtr("Network"), Seconds: 60},
			},
		},
		ScreenTime: map[string]int{"kind1": 60},
	}

	if err := store.Record(report2); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	days, err := store.ListDays()
	if err != nil {
		t.Fatalf("ListDays failed: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}

	users, err := store.GetDay(days[0])
	if err != nil {
		t.Fatalf("GetDay failed: %v", err)
	}

	ua, ok := users["kind1"]
	if !ok {
		t.Fatal("expected kind1 in activity")
	}

	firefox := ua.Apps["Firefox"]
	if firefox == nil {
		t.Fatal("expected Firefox in apps")
	}
	if firefox.Seconds != 120 {
		t.Errorf("Firefox seconds = %d, want 120", firefox.Seconds)
	}

	daemon := ua.Apps["some-daemon"]
	if daemon == nil {
		t.Fatal("expected some-daemon in apps")
	}
	if daemon.Seconds != 120 {
		t.Errorf("some-daemon seconds = %d, want 120", daemon.Seconds)
	}

	if ua.ScreenTime != 240 {
		t.Errorf("ScreenTime = %d, want 240", ua.ScreenTime)
	}
}

func TestActivityStore_MultipleHosts(t *testing.T) {
	store := newTestStore(t)

	store.Record(Report{
		Hostname: "pc1",
		Users: map[string][]AppTime{
			"kind1": {{Name: "Firefox", Category: strPtr("Network"), Seconds: 60}},
		},
	})
	store.Record(Report{
		Hostname: "pc2",
		Users: map[string][]AppTime{
			"kind1": {{Name: "Firefox", Category: strPtr("Network"), Seconds: 120}},
		},
	})

	days, _ := store.ListDays()
	users, _ := store.GetDay(days[0])

	ua := users["kind1"]
	if ua.Apps["Firefox"].Seconds != 180 {
		t.Errorf("expected 180 aggregated seconds, got %d", ua.Apps["Firefox"].Seconds)
	}
}

func TestActivityStore_CategoryGrouping(t *testing.T) {
	store := newTestStore(t)

	store.Record(Report{
		Hostname: "pc1",
		Users: map[string][]AppTime{
			"kind1": {
				{Name: "Firefox", Category: strPtr("Network"), Seconds: 60},
				{Name: "Chrome", Category: strPtr("Network"), Seconds: 30},
				{Name: "unknown-tool", Category: nil, Seconds: 10},
			},
		},
	})

	days, _ := store.ListDays()
	users, _ := store.GetDay(days[0])

	ua := users["kind1"]
	if len(ua.ByCategory["Network"]) != 2 {
		t.Errorf("expected 2 Network apps, got %d", len(ua.ByCategory["Network"]))
	}
	if len(ua.ByCategory["Other"]) != 1 {
		t.Errorf("expected 1 Other app, got %d", len(ua.ByCategory["Other"]))
	}
	if ua.ByCategory["Network"][0].Name != "Firefox" {
		t.Error("Firefox should be first (more seconds)")
	}
}

func TestActivityStore_EmptyDay(t *testing.T) {
	store := newTestStore(t)

	users, err := store.GetDay("2099-01-01")
	if err != nil {
		t.Fatalf("GetDay failed: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected empty, got %d users", len(users))
	}
}

func TestActivityStore_ChartQueries(t *testing.T) {
	store := newTestStore(t)

	// Insert some test data directly
	db := store.DB()
	db.Exec(`INSERT INTO screen_time (date, hostname, user, seconds) VALUES ('2026-04-01', 'pc1', 'kind1', 3600)`)
	db.Exec(`INSERT INTO screen_time (date, hostname, user, seconds) VALUES ('2026-04-02', 'pc1', 'kind1', 7200)`)
	db.Exec(`INSERT INTO app_activity (date, hostname, user, app_name, category, seconds) VALUES ('2026-04-01', 'pc1', 'kind1', 'Firefox', 'Network', 1800)`)
	db.Exec(`INSERT INTO app_activity (date, hostname, user, app_name, category, seconds) VALUES ('2026-04-01', 'pc1', 'kind1', 'Steam', 'Game', 1200)`)
	db.Exec(`INSERT INTO app_activity (date, hostname, user, app_name, category, seconds) VALUES ('2026-04-02', 'pc1', 'kind1', 'Firefox', 'Network', 3600)`)

	// Screen time range
	st, err := store.GetScreenTimeRange("kind1", "2026-04-01", "2026-04-02")
	if err != nil {
		t.Fatalf("GetScreenTimeRange: %v", err)
	}
	if len(st) != 2 {
		t.Fatalf("expected 2 days, got %d", len(st))
	}
	if st[0].Seconds != 3600 || st[1].Seconds != 7200 {
		t.Errorf("unexpected screen time values: %+v", st)
	}

	// App time range - all apps
	at, err := store.GetAppTimeRange("kind1", "", "2026-04-01", "2026-04-02")
	if err != nil {
		t.Fatalf("GetAppTimeRange: %v", err)
	}
	if len(at) != 3 {
		t.Fatalf("expected 3 app entries, got %d", len(at))
	}

	// App time range - single app
	at2, err := store.GetAppTimeRange("kind1", "Firefox", "2026-04-01", "2026-04-02")
	if err != nil {
		t.Fatalf("GetAppTimeRange filtered: %v", err)
	}
	if len(at2) != 2 {
		t.Fatalf("expected 2 Firefox entries, got %d", len(at2))
	}

	// List users
	users, _ := store.ListUsers()
	if len(users) != 1 || users[0] != "kind1" {
		t.Errorf("unexpected users: %v", users)
	}

	// List apps
	apps, _ := store.ListApps("kind1")
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d: %v", len(apps), apps)
	}
}
