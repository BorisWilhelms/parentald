package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
	"github.com/BorisWilhelms/parentald/internal/denylist"
	"github.com/BorisWilhelms/parentald/internal/update"
	"github.com/BorisWilhelms/parentald/internal/version"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "parentald server URL")
	interval := flag.Duration("interval", 60*time.Second, "poll interval")
	denyFile := flag.String("deny-file", "/etc/parentald/deny-users", "path to deny-users file")
	flag.Parse()

	log.SetPrefix("parentald: ")
	log.SetFlags(log.LstdFlags)

	log.Printf("starting daemon version=%s server=%s interval=%s", version.Version, *serverURL, *interval)

	tracker := activity.NewTracker(*interval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Run immediately on start
	cfg := enforce(*serverURL, *denyFile)
	trackAndReport(tracker, cfg, *serverURL)
	checkUpdate(*serverURL)

	for {
		select {
		case <-ticker.C:
			cfg := enforce(*serverURL, *denyFile)
			trackAndReport(tracker, cfg, *serverURL)
			checkUpdate(*serverURL)
		case <-stop:
			log.Println("shutting down, clearing deny file")
			denylist.Write(*denyFile, nil)
			return
		}
	}
}

func enforce(serverURL, denyFile string) config.Config {
	cfg, err := fetchConfig(serverURL)
	if err != nil {
		log.Printf("failed to fetch config: %v", err)
		return config.Config{}
	}

	now := time.Now()
	var denied []string

	for username := range cfg.Users {
		if !config.IsAllowed(username, cfg, now) {
			denied = append(denied, username)
		}
	}

	added, err := denylist.Sync(denyFile, denied)
	if err != nil {
		log.Printf("failed to sync deny file: %v", err)
		return cfg
	}

	for _, username := range added {
		log.Printf("terminating session for %s", username)
		cmd := exec.Command("loginctl", "terminate-user", username)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("loginctl terminate-user %s: %v (%s)", username, err, out)
		}
	}

	if len(denied) > 0 {
		log.Printf("denied users: %v", denied)
	}

	return cfg
}

func trackAndReport(tracker *activity.Tracker, cfg config.Config, serverURL string) {
	users := make([]string, 0, len(cfg.Users))
	for username := range cfg.Users {
		users = append(users, username)
	}

	tracker.Tick(users)

	report := tracker.Flush()
	if report == nil {
		return
	}

	if err := sendReport(serverURL, report); err != nil {
		log.Printf("failed to send activity report: %v", err)
		tracker.Merge(report)
	}
}

func sendReport(serverURL string, report *activity.Report) error {
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("%s/api/activity", serverURL), "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func checkUpdate(serverURL string) {
	updated, err := update.CheckAndUpdate(serverURL)
	if err != nil {
		log.Printf("update check failed: %v", err)
		return
	}
	if updated {
		log.Println("updated to new version, exiting for restart")
		os.Exit(0)
	}
}

func fetchConfig(serverURL string) (config.Config, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/config", serverURL))
	if err != nil {
		return config.Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return config.Config{}, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var cfg config.Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}
