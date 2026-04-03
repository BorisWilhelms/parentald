package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
)

type handlers struct {
	store    *config.Store
	actStore *activity.ActivityStore
	tmpl     *template.Template
}

func (h *handlers) index(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	if err := h.tmpl.ExecuteTemplate(w, "index.html", cfg); err != nil {
		log.Printf("template error (index.html): %v", err)
	}
}

func (h *handlers) apiConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
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
	var report activity.Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.actStore.Record(report); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) activityPage(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
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
