// Package config provides configuration loading from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the webhook server.
type Config struct {
	// TechnitiumURL is the base URL of the Technitium DNS server (e.g. http://localhost:5380).
	TechnitiumURL string
	// TechnitiumUsername is the username used for Technitium API authentication.
	TechnitiumUsername string
	// TechnitiumPassword is the password used for Technitium API authentication.
	TechnitiumPassword string
	// Zones is the list of DNS zones that this webhook manages.
	// Each record is written to the zone whose name is the longest suffix match of the record name.
	Zones []string
	// DomainFilters is the list of domains to filter; sent to external-dns during negotiation.
	DomainFilters []string
	// ListenAddress is the IP address the main webhook server binds to.
	ListenAddress string
	// ListenPort is the port for the main webhook API.
	ListenPort int
	// TechnitiumTimeout is the per-request HTTP timeout when calling the Technitium API.
	TechnitiumTimeout time.Duration
	// TechnitiumVerifySSL controls whether the Technitium server's TLS certificate is verified.
	TechnitiumVerifySSL bool
	// LogLevel controls log verbosity (DEBUG, INFO, WARN, ERROR).
	LogLevel string
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddress:       getEnvOrDefault("LISTEN_ADDRESS", "0.0.0.0"),
		ListenPort:          getEnvIntOrDefault("LISTEN_PORT", 8080),
		TechnitiumURL:       strings.TrimRight(os.Getenv("TECHNITIUM_URL"), "/"),
		TechnitiumUsername:  os.Getenv("TECHNITIUM_USERNAME"),
		TechnitiumPassword:  os.Getenv("TECHNITIUM_PASSWORD"),
		TechnitiumTimeout:   time.Duration(getEnvIntOrDefault("TECHNITIUM_TIMEOUT_SECONDS", 10)) * time.Second,
		TechnitiumVerifySSL: getEnvBoolOrDefault("TECHNITIUM_VERIFY_SSL", true),
		LogLevel:            strings.ToUpper(getEnvOrDefault("LOG_LEVEL", "INFO")),
	}

	cfg.Zones = splitList(os.Getenv("ZONES"))
	if len(cfg.Zones) == 0 {
		cfg.Zones = splitList(os.Getenv("ZONE"))
	}
	for i, z := range cfg.Zones {
		cfg.Zones[i] = NormalizeName(z)
	}
	cfg.DomainFilters = splitList(os.Getenv("DOMAIN_FILTERS"))

	var errs []string
	if cfg.TechnitiumURL == "" {
		errs = append(errs, "TECHNITIUM_URL is required")
	}
	if cfg.TechnitiumUsername == "" {
		errs = append(errs, "TECHNITIUM_USERNAME is required")
	}
	if cfg.TechnitiumPassword == "" {
		errs = append(errs, "TECHNITIUM_PASSWORD is required")
	}
	if len(cfg.Zones) == 0 {
		errs = append(errs, "ZONES (or ZONE) is required")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration errors: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}

// NormalizeName lower-cases a DNS name and strips surrounding whitespace and the trailing dot.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
}

// splitList splits a semicolon-separated list, dropping empty entries.
func splitList(v string) []string {
	var out []string
	for _, f := range strings.Split(v, ";") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBoolOrDefault(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}
