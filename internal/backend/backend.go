// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package backend provides an abstraction layer for GeoIP lookup backends.
//
// The Backend interface defines the contract that all GeoIP backends must implement.
// This allows cdnsbl to support multiple data sources for country lookups.
//
// Available implementations:
//   - geoip: MaxMind GeoLite2 database (requires license key)
//   - dbip: DB-IP Lite database (no registration required)
//   - ipapi: IP-API.com HTTP API (no registration, rate-limited)
//
// Usage:
//
//	backend, err := backend.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer backend.Close()
//
//	countryCode, err := backend.LookupCountry(ip)
package backend

import (
	"net"
)

// Backend defines the interface for GeoIP lookup implementations.
//
// All backends must implement country lookup and proper cleanup.
type Backend interface {
	// LookupCountry returns the ISO 3166-1 alpha-2 country code for the given IP.
	// Returns an error if the lookup fails or the IP is not found in the database.
	LookupCountry(ip net.IP) (string, error)

	// Close releases any resources held by the backend.
	// Should be called when the backend is no longer needed.
	Close() error
}
