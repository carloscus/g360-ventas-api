package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Addr      string
	DBPath    string
	ExportDir string
	User      string
	Pass      string
	Secret    string
	TokenTTL  time.Duration
	MaxFolios int
	MaxLimit  int
}

type producerConfig struct {
	Intranet struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	} `json:"intranet"`
}

func DataDir() string {
	if d := os.Getenv("G360_DATA_DIR"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(base, "g360-db-ventas", "data")
}

func Load() *Config {
	dataDir := DataDir()

	cfg := &Config{
		Addr:      "0.0.0.0:8090",
		DBPath:    filepath.Join(dataDir, "historial.db"),
		ExportDir: filepath.Join(dataDir, "export"),
		TokenTTL:  24 * time.Hour,
		MaxFolios: 500,
		MaxLimit:  5000,
	}

	if v := os.Getenv("G360_API_ADDR"); v != "" {
		cfg.Addr = v
	} else if v := os.Getenv("G360_API_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Addr = "0.0.0.0:" + strconv.Itoa(p)
		}
	}
	if v := os.Getenv("G360_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("G360_EXPORT_DIR"); v != "" {
		cfg.ExportDir = v
	}
	if v := os.Getenv("G360_TOKEN_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.TokenTTL = d
		}
	}
	if v := os.Getenv("G360_MAX_FOLIOS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxFolios = n
		}
	}
	if v := os.Getenv("G360_MAX_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxLimit = n
		}
	}
	if v := os.Getenv("G360_API_SECRET"); v != "" {
		cfg.Secret = v
	} else {
		cfg.Secret = randomSecret()
	}

	cfg.User, cfg.Pass = loadCreds(dataDir)
	return cfg
}

func loadCreds(dataDir string) (string, string) {
	if u := os.Getenv("G360_INTRANET_USER"); u != "" {
		p := os.Getenv("G360_INTRANET_PASS")
		return u, p
	}
	path := os.Getenv("G360_PRODUCER_CONFIG")
	if path == "" {
		path = filepath.Join(dataDir, "config.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var pc producerConfig
	if err := json.Unmarshal(raw, &pc); err != nil {
		return "", ""
	}
	return pc.Intranet.User, pc.Intranet.Pass
}

func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "g360-insecure-ephemeral-secret"
	}
	return hex.EncodeToString(b)
}
