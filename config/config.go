package config

import (
	"github.com/BurntSushi/toml"
	"golang.org/x/exp/slices"
	"path/filepath"
)

// MetadataSource is a metadata source Name.
type MetadataSource string

const (
	// MetadataSourceLiteral is the literal metadata source Name (meta.NewLiteralSource).
	MetadataSourceLiteral MetadataSource = "literal"
	// MetadataSourceAnalysis is the analysis metadata source Name (meta.NewFileAnalysisSource).
	MetadataSourceAnalysis MetadataSource = "analysis"
	// MetadataSourceTMDB is the TMDB (The Movie Database) metadata source Name (tmdb.NewSource).
	MetadataSourceTMDB MetadataSource = "tmdb"
)

// Capability is a capability Name.
type Capability string

const (
	// CapabilityWatch is the filesystem watch capability Name.
	CapabilityWatch Capability = "watch"
	// CapabilityRemux is the remux capability Name.
	CapabilityRemux Capability = "remux"
	// CapabilityTranscode is the transcode capability Name.
	CapabilityTranscode Capability = "transcode"
)

// Section is a section of the configuration file.
// T is always going to be the type of this section.
type Section[T any] interface {
	// Defaults completes the section with default values, set values are not replaced.
	Defaults() T
}

// Config is a struct representation of the TOML configuration file.
type Config struct {
	// HTTP is the "http" configuration section.
	HTTP *HTTP `toml:"http"`
	// Repos is the collection of repository configuration, keyed by their Name.
	Repos map[string]*Repo `toml:"repos"`
}

// Defaults completes the configuration with default values.
func (c *Config) Defaults() *Config {
	c.HTTP = c.HTTP.Defaults()
	for k, v := range c.Repos {
		def := v.Defaults()
		if def.Name == "" {
			def.Name = k
		}

		c.Repos[k] = def
	}

	return c
}

// HTTP is an HTTP configuration section of the configuration file.
type HTTP struct {
	// Host is the host string, used for http.ListenAndServe, defaults to ":8000".
	Host string `toml:"host"`
}

// Defaults completes the section with default values.
func (h *HTTP) Defaults() *HTTP {
	if h.Host == "" {
		h.Host = ":8000"
	}

	return h
}

// Repo is a base repository configuration.
type Repo struct {
	// Name is the name of the repository, defaults to the repository Name.
	Name string `toml:"name"`
	// Path is the relative or absolute path of the repository's directory.
	Path string `toml:"path"`
	// Path is the relative or absolute path of the repository's index file, can be empty.
	IndexPath string `toml:"index_path"`
	// CachePath is the relative or absolute path of the repository's operation cache, defaults to <path>/.katana/cache.
	CachePath string `toml:"cache_path"`
	// Capabilities are the capability IDs of the repository.
	Capabilities []Capability `toml:"capabilities"`
	// Sources is a mapping of used metadata sources and their configuration, keyed by their name.
	Sources map[MetadataSource]map[string]interface{} `toml:"sources"`
}

// Capable checks whether a Capability is contained in the configuration.
func (r *Repo) Capable(c Capability) bool {
	return slices.Contains(r.Capabilities, c)
}

// Defaults completes the section with default values.
func (r *Repo) Defaults() *Repo {
	if r.CachePath == "" {
		r.CachePath = filepath.Join(r.Path, ".katana", "cache")
	}

	return r
}

// Parse parses the configuration from a file.
func Parse(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(filepath.Clean(path), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ParseWithDefaults parses the configuration from a file and completes it with default values (Section.Defaults).
func ParseWithDefaults(path string) (*Config, error) {
	cfg, err := Parse(path)
	if err != nil {
		return nil, err
	}

	return cfg.Defaults(), nil
}
