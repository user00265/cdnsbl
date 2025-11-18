// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

package backend

import (
	"fmt"

	"github.com/user00265/cdnsbl/internal/config"
	"github.com/user00265/cdnsbl/internal/dbip"
	"github.com/user00265/cdnsbl/internal/geoip"
	"github.com/user00265/cdnsbl/internal/ip2location"
	"github.com/user00265/cdnsbl/internal/ipapi"
	"github.com/user00265/cdnsbl/internal/ipinfo"
)

// New creates a new backend instance based on the configuration.
//
// The backend type is determined by cfg.Backend:
//   - "geoip": MaxMind GeoLite2 backend
//   - "dbip": DB-IP Lite backend
//   - "ipapi": IP-API.com HTTP API backend
//   - "ipinfo": IPInfo.io backend (MMDB or HTTP API)
//   - "ip2location": IP2Location LITE backend
//
// Returns an error if the backend type is unknown or initialization fails.
func New(cfg *config.Config) (Backend, error) {
	switch cfg.Backend {
	case "geoip":
		return geoip.NewClient(cfg)
	case "dbip":
		return dbip.NewClient(cfg)
	case "ipapi":
		return ipapi.NewClient(cfg)
	case "ipinfo":
		return ipinfo.NewClient(cfg)
	case "ip2location":
		return ip2location.NewClient(cfg)
	default:
		return nil, fmt.Errorf("unknown backend: %s", cfg.Backend)
	}
}
