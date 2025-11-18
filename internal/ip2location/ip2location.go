// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package ip2location implements a GeoIP backend using IP2Location LITE.
//
// IP2Location provides free databases (DB1-DB11) with various data levels.
// The LITE DB1 (country only) is free and requires registration.
//
// Get a free account at: https://lite.ip2location.com/sign-up
// Download DB1.LITE: https://lite.ip2location.com/database/ip-country
//
// Configuration:
//   - IP2LOCATION_TOKEN: Your IP2Location download token (required for auto-download)
//   - DATA_DIR: Directory for database files (default: "./data")
//
// Note: Unlike MMDB databases, IP2Location uses BIN format which requires
// their specific library. Monthly updates are available.
package ip2location

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/user00265/cdnsbl/internal/config"
	"github.com/user00265/cdnsbl/internal/logger"
)

type Client struct {
	db         *ip2location.DB
	dbPath     string
	token      string
	httpClient *http.Client
	stopUpdate chan struct{}
	cfg        *config.Config
	log        *slog.Logger
}

// NewClient creates a new IP2Location client.
//
// Automatically downloads the database if not present and token is provided.
func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		dbPath: filepath.Join(cfg.DataDir, "ip2location-country.bin"),
		token:  cfg.IP2LocationToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cfg:        cfg,
		log:        logger.Component("ip2location"),
		stopUpdate: make(chan struct{}),
	}

	if err := client.ensureDatabase(); err != nil {
		return nil, fmt.Errorf("failed to ensure database: %w", err)
	}

	db, err := ip2location.OpenDB(client.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open IP2Location database: %w", err)
	}

	client.db = db
	client.log.Info("IP2Location database loaded", "path", client.dbPath)

	// Start background updater - IP2Location updates monthly
	go client.updateLoop(30 * 24 * time.Hour)

	return client, nil
}

func (c *Client) ensureDatabase() error {
	if _, err := os.Stat(c.dbPath); err == nil {
		info, _ := os.Stat(c.dbPath)
		age := time.Since(info.ModTime())
		if age < 30*24*time.Hour {
			c.log.Info("Using existing IP2Location database", "age", formatDuration(age))
			return nil
		}
		c.log.Info("IP2Location database is old, downloading new version", "age", formatDuration(age))
	}

	if c.token == "" {
		return fmt.Errorf("database not found and IP2LOCATION_TOKEN not set for auto-download")
	}

	return c.downloadDatabase()
}

func (c *Client) downloadDatabase() error {
	c.log.Info("Downloading IP2Location DB1 LITE database...")

	if err := os.MkdirAll(filepath.Dir(c.dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// IP2Location LITE DB1 BIN IPv4 download URL
	// Note: The URL format may vary - users may need to manually download
	url := fmt.Sprintf("https://www.ip2location.com/download/?token=%s&file=DB1LITEBINIPV6", c.token)

	c.log.Info("Downloading from IP2Location", "url_masked", "https://www.ip2location.com/download/?token=***")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s (you may need to download manually from https://lite.ip2location.com)", resp.Status)
	}

	tmpPath := c.dbPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write database: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, c.dbPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move database: %w", err)
	}

	c.log.Info("IP2Location database downloaded successfully")
	return nil
}

// LookupCountry returns the ISO 3166-1 alpha-2 country code for the given IP.
func (c *Client) LookupCountry(ip net.IP) (string, error) {
	results, err := c.db.Get_all(ip.String())
	if err != nil {
		return "", fmt.Errorf("lookup failed: %w", err)
	}

	if results.Country_short == "-" || results.Country_short == "" {
		return "", fmt.Errorf("country not found for IP: %s", ip.String())
	}

	return results.Country_short, nil
}

// Close releases resources.
func (c *Client) Close() error {
	if c.stopUpdate != nil {
		close(c.stopUpdate)
	}
	if c.db != nil {
		c.db.Close()
	}
	return nil
}

// updateLoop periodically checks for and downloads database updates.
func (c *Client) updateLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.log.Info("Checking for IP2Location database updates...")
			if err := c.updateDatabase(); err != nil {
				c.log.Error("Failed to update IP2Location database", "error", err)
			}
		case <-c.stopUpdate:
			return
		}
	}
}

// updateDatabase downloads a new database and reloads it if successful.
func (c *Client) updateDatabase() error {
	info, err := os.Stat(c.dbPath)
	if err == nil {
		age := time.Since(info.ModTime())
		if age < 25*24*time.Hour {
			c.log.Debug("IP2Location database is recent, skipping update", "age", formatDuration(age))
			return nil
		}
	}

	if c.token == "" {
		c.log.Debug("No token configured, skipping auto-update")
		return nil
	}

	if err := c.downloadDatabase(); err != nil {
		return err
	}

	newDB, err := ip2location.OpenDB(c.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open updated database: %w", err)
	}

	oldDB := c.db
	c.db = newDB

	if oldDB != nil {
		oldDB.Close()
	}

	c.log.Info("IP2Location database updated and reloaded successfully")
	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	days := d.Hours() / 24
	return fmt.Sprintf("%.1fd", days)
}
