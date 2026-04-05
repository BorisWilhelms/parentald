package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
	"github.com/BorisWilhelms/parentald/internal/crypto"
	"github.com/BorisWilhelms/parentald/internal/server"
	"github.com/BorisWilhelms/parentald/internal/version"
)

func envOrDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func main() {
	configPath := flag.String("config", envOrDefault("CONFIG_PATH", "config.json"), "path to config file")
	listen := flag.String("listen", envOrDefault("LISTEN", ":8080"), "listen address")
	adminUser := flag.String("admin-user", envOrDefault("ADMIN_USER", "admin"), "admin username")
	adminPass := flag.String("admin-pass", envOrDefault("ADMIN_PASS", ""), "admin password (required)")
	binDir := flag.String("bin-dir", envOrDefault("BIN_DIR", "dist"), "directory containing daemon binaries")
	dataDir := flag.String("data-dir", envOrDefault("DATA_DIR", "data"), "directory for activity data and keys")
	apiKey := flag.String("api-key", envOrDefault("API_KEY", ""), "API key for Home Assistant integration (optional)")
	flag.Parse()

	if *adminPass == "" {
		log.Fatal("--admin-pass or ADMIN_PASS is required")
	}

	store := config.NewStore(*configPath)
	if err := store.Load(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Load or generate server keypair
	serverPub, serverPriv, err := crypto.LoadOrGenerateKeypair(*dataDir, "server")
	if err != nil {
		log.Fatalf("failed to load/generate server keypair: %v", err)
	}
	log.Printf("server public key fingerprint: %s", crypto.Fingerprint(serverPub))

	// Load registered client keys
	clientsDir := filepath.Join(*dataDir, "clients")
	clients, err := crypto.LoadRegisteredClients(clientsDir)
	if err != nil {
		log.Fatalf("failed to load client keys: %v", err)
	}
	log.Printf("loaded %d registered client(s)", len(clients))

	// Migrate JSON activity data to SQLite (if needed)
	actStore := activity.NewActivityStore(*dataDir)
	jsonDir := filepath.Join(*dataDir, "activity")
	if err := activity.MigrateFromJSON(jsonDir, actStore.DB()); err != nil {
		log.Printf("warning: JSON migration failed: %v", err)
	}

	if *apiKey != "" {
		log.Printf("API key authentication enabled")
	}

	handler := server.New(server.ServerConfig{
		ActStore: actStore,
		Store:      store,
		AdminUser:  *adminUser,
		AdminPass:  *adminPass,
		BinDir:     *binDir,
		DataDir:    *dataDir,
		APIKey:     *apiKey,
		ServerPub:  serverPub,
		ServerPriv: serverPriv,
		Clients:    clients,
		ClientsDir: clientsDir,
	})

	log.Printf("parentald-server %s listening on %s", version.Version, *listen)
	if err := http.ListenAndServe(*listen, handler); err != nil {
		log.Fatal(err)
	}
}
