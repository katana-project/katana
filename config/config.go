package config

import (
	"github.com/BurntSushi/toml"
	"path/filepath"
)

// Config is a struct representation of the TOML configuration file.
type Config struct {
	// HTTP is the "http" configuration section.
	HTTP HTTP `toml:"http"`
	// Repos is the collection of repository configuration, keyed by their ID.
	Repos map[string]*Repo `toml:"repos"`
}

// HTTP is an HTTP configuration section of the configuration file.
type HTTP struct {
	// Host is the host string, used for http.ListenAndServe.
	Host string `toml:"host"`
}

// Repo is a base repository configuration.
type Repo struct {
	// ID is the name of the repository, can be empty.
	Name string `toml:"name"`
	// Path is the relative or absolute path of the repository's directory.
	Path string `toml:"path"`
	// Path is the relative or absolute path of the repository's index file, can be empty.
	IndexPath string `toml:"index_path"`
	// Watch is whether the repository should be assigned a filesystem watcher.
	Watch bool `toml:"watch"`
	// Sources is a mapping of used metadata sources and their configuration, keyed by their name.
	Sources map[string]map[string]interface{} `toml:"sources"`
}

// Parse parses the configuration from a file.
func Parse(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(filepath.Clean(path), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
