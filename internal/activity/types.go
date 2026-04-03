package activity

// Report is the wire format sent from daemon to server on each tick.
type Report struct {
	Hostname string                `json:"hostname"`
	Users    map[string][]AppTime `json:"users"`
}

// AppTime represents the active time for a single application.
type AppTime struct {
	Name     string  `json:"name"`
	Category *string `json:"category,omitempty"`
	Seconds  int     `json:"seconds"`
}

// AppInfo holds display information resolved from a .desktop file.
type AppInfo struct {
	Name     string
	Category *string
}
