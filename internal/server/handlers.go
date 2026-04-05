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
	serverPub  ed25519.PublicKey
	serverPriv ed25519.PrivateKey
	clientsMu  sync.RWMutex
	clients    []ed25519.PublicKey
	clientsDir string
	apiKey     string
}

// render executes a template with language support injected.
func (h *handlers) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	lang := getLang(r)
	wrapped := map[string]any{"Lang": lang, "Data": data}
	if err := h.tmpl.ExecuteTemplate(w, name, wrapped); err != nil {
		log.Printf("template error (%s): %v", name, err)
	}
}

func (h *handlers) index(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	h.render(w, r, "index.html", cfg)
}

func (h *handlers) loginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "login.html", nil)
}

func (h *handlers) loginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	lang := getLang(r)
	if username != h.adminUser || password != h.adminPass {
		wrapped := map[string]any{"Lang": lang, "Data": map[string]string{"Error": t(lang, "login.error")}}
		if err := h.tmpl.ExecuteTemplate(w, "login.html", wrapped); err != nil {
			log.Printf("template error (login.html): %v", err)
		}
		return
	}

	http.SetCookie(w, createSessionCookie(username, h.adminPass))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, clearSessionCookie())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *handlers) setLang(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("lang")
	if lang != "en" && lang != "de" {
		lang = "en"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// Alias for convenience
type configResponse = config.ConfigResponse

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

func (h *handlers) apiStatus(w http.ResponseWriter, r *http.Request) {
	statuses := h.actStore.GetStatuses()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
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

	h.renderUserList(w, r)
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

	h.renderUserList(w, r)
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

	h.renderUserSchedules(w, r, name)
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

	h.renderUserSchedules(w, r, name)
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

	h.renderUserList(w, r)
}

func (h *handlers) unlockUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	err := h.store.Update(func(cfg *config.Config) {
		user := cfg.Users[name]
		user.LockedUntil = nil
		cfg.Users[name] = user
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderUserList(w, r)
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

	h.renderUserList(w, r)
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

	h.render(w, r, "activity.html", data)
}

func (h *handlers) renderUserList(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	h.render(w, r, "user_list.html", cfg)
}

func (h *handlers) renderUserSchedules(w http.ResponseWriter, r *http.Request, username string) {
	cfg := h.store.Get()
	data := struct {
		Name string
		User config.User
	}{
		Name: username,
		User: cfg.Users[username],
	}
	h.render(w, r, "user_schedules.html", data)
}
