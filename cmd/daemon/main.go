package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
	"github.com/BorisWilhelms/parentald/internal/crypto"
	"github.com/BorisWilhelms/parentald/internal/denylist"
	"github.com/BorisWilhelms/parentald/internal/update"
	"github.com/BorisWilhelms/parentald/internal/version"
)

const configCachePath = "/etc/parentald/config-cache.json"

// Alias for convenience
type configResponse = config.ConfigResponse

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "parentald server URL")
	interval := flag.Duration("interval", 60*time.Second, "poll interval")
	denyFile := flag.String("deny-file", "/etc/parentald/deny-users", "path to deny-users file")
	keyDir := flag.String("key-dir", "/etc/parentald", "directory containing client keys and server pubkey")
	flag.Parse()

	log.SetPrefix("parentald: ")
	log.SetFlags(log.LstdFlags)

	log.Printf("starting daemon version=%s server=%s interval=%s", version.Version, *serverURL, *interval)

	// Load keys
	serverPub, err := crypto.LoadPublicKey(*keyDir + "/server.pub")
	if err != nil {
		log.Printf("warning: could not load server public key: %v (signature verification disabled)", err)
	}

	// Load or generate client keypair
	clientPub, clientPriv, err := crypto.LoadOrGenerateKeypair(*keyDir, "client")
	if err != nil {
		log.Printf("warning: could not load/generate client keypair: %v (activity signing disabled)", err)
	} else {
		log.Printf("client public key fingerprint: %s", crypto.Fingerprint(clientPub))
		// Auto-register with server if not yet registered
		registerClient(*serverURL, clientPub)
	}

	var tracker *activity.Tracker

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Run immediately on start
	resp, trusted := fetchAndVerifyConfig(*serverURL, serverPub)
	enforce(resp.Config, *denyFile)
	tracker = ensureTracker(tracker, resp.Config, *interval)
	if trusted {
		trackAndReport(tracker, resp.Config, *serverURL, clientPriv, clientPub)
		checkUpdate(*serverURL, resp.Version, serverPub)
	} else {
		tracker.Tick(usernames(resp.Config))
	}

	for {
		select {
		case <-ticker.C:
			resp, trusted := fetchAndVerifyConfig(*serverURL, serverPub)
			enforce(resp.Config, *denyFile)
			tracker = ensureTracker(tracker, resp.Config, *interval)
			if trusted {
				trackAndReport(tracker, resp.Config, *serverURL, clientPriv, clientPub)
				checkUpdate(*serverURL, resp.Version, serverPub)
			} else {
				tracker.Tick(usernames(resp.Config))
			}
		case <-stop:
			log.Println("shutting down, clearing deny file")
			denylist.Write(*denyFile, nil)
			return
		}
	}
}

func registerClient(serverURL string, clientPub ed25519.PublicKey) {
	body := fmt.Sprintf(`{"publicKey":"%s"}`, crypto.EncodePublicKey(clientPub))
	resp, err := http.Post(
		fmt.Sprintf("%s/api/register", serverURL),
		"application/json",
		bytes.NewReader([]byte(body)),
	)
	if err != nil {
		log.Printf("failed to register with server: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		log.Printf("registered with server")
	} else if resp.StatusCode == http.StatusOK {
		log.Printf("already registered with server")
	} else {
		log.Printf("registration returned %d", resp.StatusCode)
	}
}

func usernames(cfg config.Config) []string {
	users := make([]string, 0, len(cfg.Users))
	for u := range cfg.Users {
		users = append(users, u)
	}
	return users
}

// fetchAndVerifyConfig fetches config from server and verifies the signature.
// Falls back to cache on failure. Returns the config and whether the server was trusted.
func fetchAndVerifyConfig(serverURL string, serverPub ed25519.PublicKey) (configResponse, bool) {
	resp, err := http.Get(fmt.Sprintf("%s/api/config", serverURL))
	if err != nil {
		log.Printf("failed to fetch config: %v", err)
		return loadConfigCache(), false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("config request failed: %d", resp.StatusCode)
		return loadConfigCache(), false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read config response: %v", err)
		return loadConfigCache(), false
	}

	// Verify signature if we have the server's public key
	if serverPub != nil {
		sig := resp.Header.Get("X-Signature")
		if sig == "" || !crypto.Verify(body, sig, serverPub) {
			log.Printf("config signature verification failed, using cache")
			return loadConfigCache(), false
		}
	}

	var cr configResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		log.Printf("failed to parse config: %v", err)
		return loadConfigCache(), false
	}

	// Write cache
	writeConfigCache(body)

	return cr, true
}

func loadConfigCache() configResponse {
	data, err := os.ReadFile(configCachePath)
	if err != nil {
		log.Printf("no config cache available: %v", err)
		return configResponse{}
	}
	var cr configResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		log.Printf("failed to parse config cache: %v", err)
		return configResponse{}
	}
	log.Printf("using cached config")
	return cr
}

func writeConfigCache(data []byte) {
	if err := os.WriteFile(configCachePath, data, 0600); err != nil {
		log.Printf("failed to write config cache: %v", err)
	}
}

func enforce(cfg config.Config, denyFile string) {
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
		return
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
}

func ensureTracker(tracker *activity.Tracker, cfg config.Config, interval time.Duration) *activity.Tracker {
	if tracker != nil {
		return tracker
	}
	return activity.NewTracker(interval, usernames(cfg))
}

func trackAndReport(tracker *activity.Tracker, cfg config.Config, serverURL string, clientPriv ed25519.PrivateKey, clientPub ed25519.PublicKey) {
	tracker.Tick(usernames(cfg))

	report := tracker.Flush()
	if report == nil {
		return
	}

	if err := sendReport(serverURL, report, clientPriv, clientPub); err != nil {
		log.Printf("failed to send activity report: %v", err)
		tracker.Merge(report)
	}
}

func sendReport(serverURL string, report *activity.Report, clientPriv ed25519.PrivateKey, clientPub ed25519.PublicKey) error {
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/activity", serverURL), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Sign the request body
	if clientPriv != nil {
		req.Header.Set("X-Signature", crypto.Sign(data, clientPriv))
		req.Header.Set("X-Public-Key", crypto.EncodePublicKey(clientPub))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func checkUpdate(serverURL, remoteVersion string, serverPub ed25519.PublicKey) {
	updated, err := update.CheckAndUpdate(serverURL, remoteVersion, serverPub)
	if err != nil {
		log.Printf("update check failed: %v", err)
		return
	}
	if updated {
		log.Println("updated to new version, exiting for restart")
		os.Exit(0)
	}
}
