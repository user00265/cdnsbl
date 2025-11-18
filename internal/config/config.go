// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package config handles application configuration loading from environment variables.
//
// Configuration is loaded from environment variables with sensible defaults.
// The godotenv package is used in main to support .env files.
//
// Environment variables:
//   - BACKEND: Backend selection (default: "dbip")
//   - DATA_DIR: Directory for database files (default: "./data" or "/data" in Docker)
//   - DNS_LISTEN_ADDR: DNS server listen address (default: ":5353")
//   - DNS_ZONE: DNS zone to serve (default: "cbl.home.lan")
//   - GEOIP_LICENSE_KEY: MaxMind license key (required for geoip backend)
//   - GEOIP_ACCOUNT_ID: MaxMind account ID (required for geoip backend)
//   - IPINFO_TOKEN: IPInfo.io API token (required for ipinfo backend)
//   - IPINFO_USE_API: Force API mode instead of database (default: false)
//   - IP2LOCATION_TOKEN: IP2Location download token (optional, for auto-download)
package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration.
type Config struct {
	Backend          string // Backend selection: "geoip", "dbip", "ipapi", "ipinfo", or "ip2location"
	DataDir          string // Directory for database files
	GeoIPLicenseKey  string // MaxMind GeoIP license key
	GeoIPAccountID   string // MaxMind GeoIP account ID
	IPInfoToken      string // IPInfo.io API token
	IPInfoUseAPI     bool   // Force API mode for IPInfo instead of database
	IP2LocationToken string // IP2Location download token
	DNSListenAddr    string // DNS server listen address (e.g., ":53" or ":5353")
	DNSZone          string // DNS zone to serve queries for
}

// Load reads configuration from environment variables and validates required fields.
//
// Returns an error if required configuration is missing based on the selected backend.
// For the "geoip" backend, GEOIP_LICENSE_KEY and GEOIP_ACCOUNT_ID are required.
// Other backends have no additional requirements beyond the base configuration.
func Load() (*Config, error) {
	cfg := &Config{
		Backend:          getEnvOrDefault("BACKEND", "dbip"),
		DataDir:          getEnvOrDefault("DATA_DIR", "./data"),
		GeoIPLicenseKey:  os.Getenv("GEOIP_LICENSE_KEY"),
		GeoIPAccountID:   os.Getenv("GEOIP_ACCOUNT_ID"),
		IPInfoToken:      os.Getenv("IPINFO_TOKEN"),
		IPInfoUseAPI:     os.Getenv("IPINFO_USE_API") == "true",
		IP2LocationToken: os.Getenv("IP2LOCATION_TOKEN"),
		DNSListenAddr:    getEnvOrDefault("DNS_LISTEN_ADDR", ":5353"),
		DNSZone:          getEnvOrDefault("DNS_ZONE", "cbl.home.lan"),
	}

	if cfg.Backend == "geoip" {
		if cfg.GeoIPLicenseKey == "" {
			return nil, fmt.Errorf("GEOIP_LICENSE_KEY is required for geoip backend")
		}

		if cfg.GeoIPAccountID == "" {
			return nil, fmt.Errorf("GEOIP_ACCOUNT_ID is required for geoip backend")
		}
	}

	if cfg.Backend == "ipinfo" {
		if cfg.IPInfoToken == "" {
			return nil, fmt.Errorf("IPINFO_TOKEN is required for ipinfo backend")
		}
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
