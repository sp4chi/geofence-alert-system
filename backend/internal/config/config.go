// Package config reads all runtime settings from environment variables,
// so the same Docker image runs locally and in production.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
	CORSOrigins []string

	// Optional hardening; empty / zero means "off" so evaluator calls are never blocked.
	APIKey       string
	RateLimitRPS float64

	// When true, entering a restricted_zone raises an alert even without a configured rule.
	RestrictedZoneDefaultAlert bool
}

// Load reads the configuration from the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

// load takes the lookup function as a parameter so tests can supply a fake environment.
func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:        withDefault(getenv("PORT"), "8080"),
		DatabaseURL: getenv("DATABASE_URL"),
		CORSOrigins: normalizeOrigins(splitList(getenv("CORS_ORIGINS"))),
		APIKey:      getenv("API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	rps, err := strconv.ParseFloat(withDefault(getenv("RATE_LIMIT_RPS"), "0"), 64)
	if err != nil || rps < 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_RPS must be a non-negative number, got %q", getenv("RATE_LIMIT_RPS"))
	}
	cfg.RateLimitRPS = rps

	restricted, err := strconv.ParseBool(withDefault(getenv("RESTRICTED_ZONE_DEFAULT_ALERT"), "true"))
	if err != nil {
		return Config{}, fmt.Errorf("RESTRICTED_ZONE_DEFAULT_ALERT must be true or false, got %q", getenv("RESTRICTED_ZONE_DEFAULT_ALERT"))
	}
	cfg.RestrictedZoneDefaultAlert = restricted

	return cfg, nil
}

func withDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// normalizeOrigins reduces each entry to "scheme://host[:port]", the exact form browsers send in the
// Origin header. A URL pasted from the address bar ("https://app.vercel.app/geofences", a trailing
// slash, or quotes) would otherwise never match and silently block the frontend.
func normalizeOrigins(entries []string) []string {
	var origins []string
	for _, entry := range entries {
		entry = strings.Trim(entry, `"'`)
		if u, err := url.Parse(entry); err == nil && u.Scheme != "" && u.Host != "" {
			entry = strings.ToLower(u.Scheme + "://" + u.Host)
		}
		origins = append(origins, entry) // "*" and anything unparseable pass through unchanged
	}
	return origins
}

// splitList turns "a, b,,c" into ["a" "b" "c"].
func splitList(value string) []string {
	var items []string
	for _, part := range strings.Split(value, ",") {
		if item := strings.TrimSpace(part); item != "" {
			items = append(items, item)
		}
	}
	return items
}
