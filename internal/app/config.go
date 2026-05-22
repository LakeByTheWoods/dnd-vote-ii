package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	defaultHost                = "0.0.0.0"
	defaultPort                = "8000"
	defaultDataDir             = "data"
	defaultDatabaseFile        = "dnd_vote.db"
	defaultMaxRequestBodyBytes = int64(8192)
)

type Config struct {
	Host                string
	Port                string
	DataDir             string
	DatabasePath        string
	AdminSecret         string
	StaticDir           string
	MaxRequestBodyBytes int64
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
}

func DefaultConfig() Config {
	return Config{
		Host:                defaultHost,
		Port:                defaultPort,
		DataDir:             defaultDataDir,
		DatabasePath:        filepath.Join(defaultDataDir, defaultDatabaseFile),
		StaticDir:           ".",
		MaxRequestBodyBytes: defaultMaxRequestBodyBytes,
		ReadHeaderTimeout:   5 * time.Second,
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        10 * time.Second,
		IdleTimeout:         60 * time.Second,
	}
}

func LoadConfig() (Config, error) {
	return LoadConfigFromLookup(os.LookupEnv)
}

func LoadConfigFromLookup(lookup func(string) (string, bool)) (Config, error) {
	cfg := DefaultConfig()

	cfg.Host = lookupFirst(lookup, cfg.Host, "DND_VOTE_HOST", "HOST")
	cfg.Port = lookupFirst(lookup, cfg.Port, "DND_VOTE_PORT", "PORT")
	cfg.DataDir = lookupFirst(lookup, cfg.DataDir, "DND_VOTE_DATA_DIR")
	cfg.AdminSecret = lookupFirst(lookup, cfg.AdminSecret, "DND_VOTE_ADMIN_SECRET")
	cfg.StaticDir = lookupFirst(lookup, cfg.StaticDir, "DND_VOTE_STATIC_DIR")

	if databasePath, ok := lookup("DND_VOTE_DB_PATH"); ok && databasePath != "" {
		cfg.DatabasePath = databasePath
	} else {
		cfg.DatabasePath = filepath.Join(cfg.DataDir, defaultDatabaseFile)
	}

	if value, ok := lookup("DND_VOTE_MAX_REQUEST_BODY_BYTES"); ok && value != "" {
		maxBodyBytes, err := strconv.ParseInt(value, 10, 64)
		if err != nil || maxBodyBytes <= 0 {
			return Config{}, fmt.Errorf("DND_VOTE_MAX_REQUEST_BODY_BYTES must be a positive integer")
		}
		cfg.MaxRequestBodyBytes = maxBodyBytes
	}

	return cfg, nil
}

func lookupFirst(lookup func(string) (string, bool), fallback string, keys ...string) string {
	for _, key := range keys {
		if value, ok := lookup(key); ok && value != "" {
			return value
		}
	}
	return fallback
}
