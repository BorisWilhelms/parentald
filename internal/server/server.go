package server

import (
	"net/http"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
)

// New creates the HTTP handler with all routes configured.
func New(store *config.Store, adminUser, adminPass, binDir, dataDir string) http.Handler {
	tmpl := parseTemplates()
	actStore := activity.NewActivityStore(dataDir + "/activity")
	h := &handlers{store: store, actStore: actStore, tmpl: tmpl}
	ih := &installHandlers{binDir: binDir}

	mux := http.NewServeMux()

	// Public API (no auth) — used by daemons and installers
	mux.HandleFunc("GET /api/config", h.apiConfig)
	mux.HandleFunc("POST /api/activity", h.apiActivity)
	mux.HandleFunc("GET /api/version", ih.serveVersion)
	mux.HandleFunc("GET /api/daemon/{os}/{arch}", ih.serveBinary)
	mux.HandleFunc("GET /install", ih.serveInstallScript)

	// Protected routes (Basic Auth) — used by admin UI
	protected := http.NewServeMux()
	protected.HandleFunc("GET /", h.index)
	protected.HandleFunc("GET /activity", h.activityPage)
	protected.HandleFunc("POST /users", h.addUser)
	protected.HandleFunc("DELETE /users/{name}", h.deleteUser)
	protected.HandleFunc("POST /users/{name}/schedules", h.addSchedule)
	protected.HandleFunc("DELETE /users/{name}/schedules/{index}", h.deleteSchedule)
	protected.HandleFunc("POST /users/{name}/lock", h.lockUser)
	protected.HandleFunc("POST /users/{name}/unlock", h.unlockUser)
	protected.HandleFunc("POST /users/{name}/bonus", h.addBonus)

	mux.Handle("/", basicAuth(protected, adminUser, adminPass))

	return mux
}
