package config

import (
	"github.com/BurntSushi/toml"
	"golang.org/x/exp/slices"
	"path/filepath"
)

// MetadataSource is a metadata source ID.
type MetadataSource string

const (
	// MetadataSourceLiteral is the literal metadata source ID (meta.NewLiteralSource).
	MetadataSourceLiteral MetadataSource = "literal"
	// MetadataSourceAnalysis is the analysis metadata source ID (meta.NewFileAnalysisSource).
	MetadataSourceAnalysis MetadataSource = "analysis"
	// MetadataSourceTMDB is the TMDB (The Movie Database) metadata source ID (tmdb.NewSource).
	MetadataSourceTMDB MetadataSource = "tmdb"
)

// Capability is a capability ID.
type Capability string

const (
	// CapabilityWatch is the filesystem watch capability ID.
	CapabilityWatch Capability = "watch"
	// CapabilityTranscode is the transcode capability ID.
	CapabilityTranscode Capability = "transcode"
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
	// MuxCachePath is the relative or absolute path of the repository's remux and transcode cache, can be empty.
	MuxCachePath string `toml:"mux_cache_path"`
	// Capabilities are the capability IDs of the repository.
	Capabilities []Capability `toml:"capabilities"`
	// Sources is a mapping of used metadata sources and their configuration, keyed by their name.
	Sources map[MetadataSource]map[string]interface{} `toml:"sources"`
}

// Capable checks whether a Capability is contained in the configuration.
func (r *Repo) Capable(c Capability) bool {
	return slices.Contains(r.Capabilities, c)
}

// Parse parses the configuration from a file.
func Parse(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(filepath.Clean(path), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
