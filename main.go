// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package main implements a country-based DNS blacklist (DNSBL) server.
//
// cdnsbl is a DNS server that returns 127.0.0.1 when queried for IPs belonging
// to specific countries, and NXDOMAIN otherwise. It's designed for integration
// with mail servers (Postfix, SpamAssassin) and firewalls for country-based filtering.
//
// Query format: <reversed-ip>.<country-code>.<zone>
// Example: 9.48.252.210.jp.cbl.home.lan checks if 210.252.48.9 is in Japan.
//
// Supported backends:
//   - geoip: MaxMind GeoLite2 (requires license key)
//   - dbip: DB-IP Lite (no registration required)
//   - ipapi: IP-API.com (no registration, rate-limited)
//
// Configuration is done via environment variables or a .env file.
// See .env.example for available options.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/user00265/cdnsbl/internal/backend"
	"github.com/user00265/cdnsbl/internal/config"
	"github.com/user00265/cdnsbl/internal/dns"
	"github.com/user00265/cdnsbl/internal/logger"
)

func main() {
	// Initialize logger first
	logger.Init()
	log := logger.Component("main")

	if err := godotenv.Load(); err != nil {
		log.Info("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	backendClient, err := backend.New(cfg)
	if err != nil {
		log.Error("Failed to initialize backend", "error", err)
		os.Exit(1)
	}
	defer func() {
		log.Info("Closing backend...")
		if err := backendClient.Close(); err != nil {
			log.Error("Error closing backend", "error", err)
		}
	}()

	dnsServer := dns.NewServer(cfg, backendClient)

	if err := dnsServer.Start(); err != nil {
		log.Error("Failed to start DNS server", "error", err)
		os.Exit(1)
	}
	defer func() {
		log.Info("Stopping DNS server...")
		dnsServer.Stop()
	}()

	log.Info("cdnsbl started",
		"address", cfg.DNSListenAddr,
		"zone", cfg.DNSZone,
		"backend", cfg.Backend)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig

	log.Info("Received signal", "signal", s.String())
	log.Info("Shutting down gracefully...")
}
