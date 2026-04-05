package server

import (
	"crypto/ed25519"
	"log"
	"net/http"
	"time"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
)

// ServerConfig holds all configuration for the HTTP server.
type ServerConfig struct {
	Store      *config.Store
	ActStore   *activity.ActivityStore
	AdminUser  string
	AdminPass  string
	BinDir     string
	DataDir    string
	APIKey     string
	ServerPub  ed25519.PublicKey
	ServerPriv ed25519.PrivateKey
	Clients    []ed25519.PublicKey
	ClientsDir string
}

// New creates the HTTP handler with all routes configured.
func New(cfg ServerConfig) http.Handler {
	tmpl := parseTemplates()
	h := &handlers{
		store:      cfg.Store,
		actStore:   cfg.ActStore,
		tmpl:       tmpl,
		adminUser:  cfg.AdminUser,
		adminPass:  cfg.AdminPass,
		apiKey:     cfg.APIKey,
		serverPub:  cfg.ServerPub,
		serverPriv: cfg.ServerPriv,
		clients:    cfg.Clients,
		clientsDir: cfg.ClientsDir,
	}
	ih := &installHandlers{binDir: cfg.BinDir, serverPub: cfg.ServerPub, serverPriv: cfg.ServerPriv}

	mux := http.NewServeMux()

	// Public API (no auth) — used by daemons and installers
	mux.HandleFunc("GET /api/config", h.apiConfig)
	mux.HandleFunc("POST /api/activity", h.apiActivity)
	mux.HandleFunc("GET /api/daemon/{os}/{arch}", ih.serveBinary)
	mux.HandleFunc("GET /api/server-pubkey", h.serverPubKey)
	mux.HandleFunc("POST /api/register", h.registerClient)
	mux.HandleFunc("GET /install", ih.serveInstallScript)

	// Static assets (no auth)
	mux.Handle("GET /manifest.json", staticHandler())
	mux.Handle("GET /icon.svg", staticHandler())
	mux.Handle("GET /chart.min.js", staticHandler())

	// Login/logout/language (no auth)
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /login", h.loginSubmit)
	mux.HandleFunc("GET /logout", h.logout)
	mux.HandleFunc("GET /lang/{lang}", h.setLang)

	// Protected routes (cookie or API key auth) — used by admin UI and HA
	protected := http.NewServeMux()
	protected.HandleFunc("GET /", h.index)
	protected.HandleFunc("GET /api/status", h.apiStatus)
	protected.HandleFunc("GET /activity", h.activityPage)
	protected.HandleFunc("GET /dashboard", h.dashboardPage)
	protected.HandleFunc("GET /api/chart/screentime", h.apiChartScreenTime)
	protected.HandleFunc("GET /api/chart/apptime", h.apiChartAppTime)
	protected.HandleFunc("GET /api/chart/users", h.apiChartUsers)
	protected.HandleFunc("GET /api/chart/apps", h.apiChartApps)
	protected.HandleFunc("POST /users", h.addUser)
	protected.HandleFunc("DELETE /users/{name}", h.deleteUser)
	protected.HandleFunc("POST /users/{name}/schedules", h.addSchedule)
	protected.HandleFunc("DELETE /users/{name}/schedules/{index}", h.deleteSchedule)
	protected.HandleFunc("POST /users/{name}/lock", h.lockUser)
	protected.HandleFunc("POST /users/{name}/unlock", h.unlockUser)
	protected.HandleFunc("POST /users/{name}/bonus", h.addBonus)

	mux.Handle("/", cookieAuth(protected, h.adminPass, h.apiKey))

	return requestLogger(mux)
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}
