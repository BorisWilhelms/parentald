package config

// ConfigResponse is the merged config + version API response.
type ConfigResponse struct {
	Version string `json:"version"`
	Config  Config `json:"config"`
}
