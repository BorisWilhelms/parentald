package server

import (
	"crypto/ed25519"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
	"github.com/BorisWilhelms/parentald/internal/crypto"
	"github.com/BorisWilhelms/parentald/internal/version"
)

type handlers struct {
	store      *config.Store
	actStore   *activity.ActivityStore
	tmpl       *template.Template
	adminUser  string
	adminPass  string
	secret     string
	serverPub  ed25519.PublicKey
	serverPriv ed25519.PrivateKey
	clientsMu  sync.RWMutex
	clients    []ed25519.PublicKey
	clientsDir string
}

func (h *handlers) index(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	if err := h.tmpl.ExecuteTemplate(w, "index.html", cfg); err != nil {
		log.Printf("template error (index.html): %v", err)
	}
}

func (h *handlers) loginPage(w http.ResponseWriter, r *http.Request) {
	h.tmpl.ExecuteTemplate(w, "login.html", nil)
}

func (h *handlers) loginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username != h.adminUser || password != h.adminPass {
		if err := h.tmpl.ExecuteTemplate(w, "login.html", map[string]string{"Error": "Ungültige Anmeldedaten"}); err != nil {
			log.Printf("template error (login.html): %v", err)
		}
		return
	}

	http.SetCookie(w, createSessionCookie(username, h.secret))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, clearSessionCookie())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// configResponse is the merged config + version endpoint response.
type configResponse struct {
	Version string        `json:"version"`
	Config  config.Config `json:"config"`
}

func (h *handlers) apiConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	resp := configResponse{
		Version: version.Version,
		Config:  cfg,
	}
	body, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sig := crypto.Sign(body, h.serverPriv)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Signature", sig)
	w.Write(body)
}

func (h *handlers) serverPubKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(crypto.EncodePublicKey(h.serverPub)))
}

func (h *handlers) registerClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string `json:"publicKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	pubKey, err := crypto.DecodePublicKey(req.PublicKey)
	if err != nil {
		http.Error(w, "invalid public key", http.StatusBadRequest)
		return
	}

	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if crypto.IsRegisteredClient(h.clients, pubKey) {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := crypto.RegisterClient(h.clientsDir, pubKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.clients = append(h.clients, pubKey)

	log.Printf("registered new client: %s", crypto.Fingerprint(pubKey))
	w.WriteHeader(http.StatusCreated)
}

func (h *handlers) addUser(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	err := h.store.Update(func(cfg *config.Config) {
		if _, exists := cfg.Users[name]; !exists {
			cfg.Users[name] = config.User{}
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserList(w)
}

func (h *handlers) deleteUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	err := h.store.Update(func(cfg *config.Config) {
		delete(cfg.Users, name)
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserList(w)
}

func (h *handlers) addSchedule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	r.ParseForm()

	days := r.Form["days"]
	from := r.FormValue("from")
	to := r.FormValue("to")

	if len(days) == 0 || from == "" || to == "" {
		http.Error(w, "days, from, and to are required", http.StatusBadRequest)
		return
	}

	schedule := config.Schedule{Days: days, From: from, To: to}

	err := h.store.Update(func(cfg *config.Config) {
		user := cfg.Users[name]
		user.Schedules = append(user.Schedules, schedule)
		cfg.Users[name] = user
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserSchedules(w, name)
}

func (h *handlers) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	idxStr := r.PathValue("index")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	err = h.store.Update(func(cfg *config.Config) {
		user := cfg.Users[name]
		if idx >= 0 && idx < len(user.Schedules) {
			user.Schedules = append(user.Schedules[:idx], user.Schedules[idx+1:]...)
			cfg.Users[name] = user
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserSchedules(w, name)
}

func (h *handlers) lockUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	now := time.Now()

	err := h.store.Update(func(cfg *config.Config) {
		user := cfg.Users[name]
		next := config.NextScheduleStart(user, now)
		if next != nil {
			user.LockedUntil = next
		} else {
			// No schedules: lock "forever" (far future)
			t := time.Date(9999, 1, 1, 0, 0, 0, 0, now.Location())
			user.LockedUntil = &t
		}
		cfg.Users[name] = user
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserList(w)
}

func (h *handlers) unlockUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	now := time.Now()

	cfg := h.store.Get()
	user := cfg.Users[name]

	// Only allow unlock if currently within a schedule window
	if !config.IsInSchedule(user.Schedules, now) {
		http.Error(w, "unlock only allowed during an active schedule", http.StatusBadRequest)
		return
	}

	err := h.store.Update(func(cfg *config.Config) {
		user := cfg.Users[name]
		user.LockedUntil = nil
		cfg.Users[name] = user
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserList(w)
}

func (h *handlers) addBonus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	r.ParseForm()

	minutesStr := r.FormValue("minutes")
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		http.Error(w, "invalid minutes", http.StatusBadRequest)
		return
	}

	now := time.Now()
	bonus := time.Duration(minutes) * time.Minute

	err = h.store.Update(func(cfg *config.Config) {
		user := cfg.Users[name]

		// Clear lock when granting bonus
		user.LockedUntil = nil

		// Compute bonus end time
		var bonusEnd time.Time
		if user.BonusUntil != nil && now.Before(*user.BonusUntil) {
			// Stack: add to existing bonus
			bonusEnd = user.BonusUntil.Add(bonus)
		} else if end := config.ScheduleEndTime(user, now); end != nil {
			// In schedule: extend past schedule end
			bonusEnd = end.Add(bonus)
		} else {
			// Outside schedule: start from now
			bonusEnd = now.Add(bonus)
		}

		user.BonusUntil = &bonusEnd
		cfg.Users[name] = user
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserList(w)
}

func (h *handlers) apiActivity(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify client signature
	sig := r.Header.Get("X-Signature")
	pubKeyHeader := r.Header.Get("X-Public-Key")
	if sig == "" || pubKeyHeader == "" {
		http.Error(w, "missing signature or public key", http.StatusUnauthorized)
		return
	}

	clientPub, err := crypto.DecodePublicKey(pubKeyHeader)
	if err != nil {
		http.Error(w, "invalid public key", http.StatusBadRequest)
		return
	}

	h.clientsMu.RLock()
	registered := crypto.IsRegisteredClient(h.clients, clientPub)
	h.clientsMu.RUnlock()

	if !registered {
		http.Error(w, "unregistered client", http.StatusForbidden)
		return
	}

	if !crypto.Verify(body, sig, clientPub) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	var report activity.Report
	if err := json.Unmarshal(body, &report); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.actStore.Record(report); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var validDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (h *handlers) activityPage(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !validDate.MatchString(date) {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}

	users, err := h.actStore.GetDay(date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Date  string
		Users map[string]*activity.UserActivity
	}{
		Date:  date,
		Users: users,
	}

	if err := h.tmpl.ExecuteTemplate(w, "activity.html", data); err != nil {
		log.Printf("template error (activity.html): %v", err)
	}
}

func (h *handlers) renderUserList(w http.ResponseWriter) {
	cfg := h.store.Get()
	if err := h.tmpl.ExecuteTemplate(w, "user_list.html", cfg); err != nil {
		log.Printf("template error (user_list.html): %v", err)
	}
}

func (h *handlers) renderUserSchedules(w http.ResponseWriter, username string) {
	cfg := h.store.Get()
	data := struct {
		Name string
		User config.User
	}{
		Name: username,
		User: cfg.Users[username],
	}
	if err := h.tmpl.ExecuteTemplate(w, "user_schedules.html", data); err != nil {
		log.Printf("template error (user_schedules.html): %v", err)
	}
}
