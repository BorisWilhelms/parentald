package activity

import (
	"path/filepath"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestActivityStore_RecordAndGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "activity")
	store := NewActivityStore(dir)

	report := Report{
		Hostname: "pc1",
		Users: map[string][]AppTime{
			"kind1": {
				{Name: "Firefox", Category: strPtr("Network"), Seconds: 60},
				{Name: "some-daemon", Category: nil, Seconds: 120},
			},
		},
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
}

func TestActivityStore_MultipleHosts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "activity")
	store := NewActivityStore(dir)

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
	dir := filepath.Join(t.TempDir(), "activity")
	store := NewActivityStore(dir)

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
		t.Errorf("expected 1 Sonstiges app, got %d", len(ua.ByCategory["Other"]))
	}
	// Sorted descending
	if ua.ByCategory["Network"][0].Name != "Firefox" {
		t.Error("Firefox should be first (more seconds)")
	}
}

func TestActivityStore_EmptyDay(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "activity")
	store := NewActivityStore(dir)

	users, err := store.GetDay("2099-01-01")
	if err != nil {
		t.Fatalf("GetDay failed: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected empty, got %d users", len(users))
	}
}
