package config

import "time"

type AppType string

const (
	AppTypeNode   AppType = "node"
	AppTypeStatic AppType = "static"
)

type AppConfig struct {
	Name        string    `json:"name"`
	Type        AppType   `json:"type"`
	Port        int       `json:"port"`
	Entry       string    `json:"entry,omitempty"`
	BuildDir    string    `json:"build_dir,omitempty"`
	EnvFile     string    `json:"env_file,omitempty"`
	WorkingDir  string    `json:"working_dir,omitempty"`
	NginxDomain string    `json:"nginx_domain,omitempty"`
	NginxPath   string    `json:"nginx_path,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Enabled     bool      `json:"enabled"`
}

type Store interface {
	// Load reads an AppConfig by name from persistent storage.
	Load(name string) (AppConfig, error)

	// Save writes an AppConfig to persistent storage, creating or overwriting.
	Save(app AppConfig) error

	// Delete removes an AppConfig by name from persistent storage.
	Delete(name string) error

	// List returns all AppConfigs currently stored.
	List() ([]AppConfig, error)

	// AppPath returns the full file path for a given app name in the storage dir.
	AppPath(name string) (string, error)
}
