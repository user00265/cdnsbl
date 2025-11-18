// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package ipinfo implements a GeoIP backend using IPInfo.io.
//
// This backend supports two modes:
//  1. MMDB database (free download, requires token for download only)
//  2. HTTP API (requires token, 50k requests/month free tier)
//
// The mode is determined automatically:
//   - If IPINFO_DB_PATH exists or IPINFO_TOKEN is set with no file, downloads MMDB
//   - Otherwise uses HTTP API (requires IPINFO_TOKEN)
//
// Get a free token at: https://ipinfo.io/signup
//
// Configuration:
//   - IPINFO_TOKEN: Your IPInfo.io API token (required)
//   - DATA_DIR: Directory for database files (default: "./data")
//   - IPINFO_USE_API: Force API mode even if database exists (optional)
package ipinfo

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/user00265/cdnsbl/internal/config"
	"github.com/user00265/cdnsbl/internal/logger"
)

type Client struct {
	db         *geoip2.Reader
	dbPath     string
	token      string
	useAPI     bool
	httpClient *http.Client
	stopUpdate chan struct{}
	cfg        *config.Config
	log        *slog.Logger
}

// NewClient creates a new IPInfo.io client.
//
// Automatically selects between MMDB database and HTTP API mode based on configuration.
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.IPInfoToken == "" {
		return nil, fmt.Errorf("IPINFO_TOKEN is required for ipinfo backend")
	}

	client := &Client{
		dbPath: filepath.Join(cfg.DataDir, "ipinfo-country.mmdb"),
		token:  cfg.IPInfoToken,
		useAPI: cfg.IPInfoUseAPI,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		cfg:        cfg,
		log:        logger.Component("ipinfo"),
		stopUpdate: make(chan struct{}),
	}

	// Try database mode first unless forced to API
	if !client.useAPI {
		if err := client.ensureDatabase(); err != nil {
			client.log.Warn("Failed to setup database, falling back to API mode", "error", err)
			client.useAPI = true
		} else {
			db, err := geoip2.Open(client.dbPath)
			if err != nil {
				client.log.Warn("Failed to open database, falling back to API mode", "error", err)
				client.useAPI = true
			} else {
				client.db = db
				client.log.Info("IPInfo database loaded", "path", client.dbPath, "mode", "MMDB")

				// Start background updater - IPInfo updates monthly
				go client.updateLoop(30 * 24 * time.Hour)
			}
		}
	}

	if client.useAPI {
		client.log.Info("IPInfo initialized", "mode", "API", "rate_limit", "50k/month")
	}

	return client, nil
}

func (c *Client) ensureDatabase() error {
	if _, err := os.Stat(c.dbPath); err == nil {
		info, _ := os.Stat(c.dbPath)
		age := time.Since(info.ModTime())
		if age < 30*24*time.Hour {
			c.log.Info("Using existing IPInfo database", "age", formatDuration(age))
			return nil
		}
		c.log.Info("IPInfo database is old, downloading new version", "age", formatDuration(age))
	}

	return c.downloadDatabase()
}

func (c *Client) downloadDatabase() error {
	c.log.Info("Downloading IPInfo Country database...")

	if err := os.MkdirAll(filepath.Dir(c.dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// IPInfo.io free database download URL (requires token)
	url := fmt.Sprintf("https://ipinfo.io/data/free/country.mmdb?token=%s", c.token)

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
		return fmt.Errorf("download failed with status: %s", resp.Status)
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

	c.log.Info("IPInfo database downloaded successfully")
	return nil
}

// LookupCountry returns the ISO 3166-1 alpha-2 country code for the given IP.
func (c *Client) LookupCountry(ip net.IP) (string, error) {
	if c.useAPI {
		return c.lookupAPI(ip)
	}
	return c.lookupDB(ip)
}

func (c *Client) lookupDB(ip net.IP) (string, error) {
	record, err := c.db.Country(ip)
	if err != nil {
		return "", err
	}
	return record.Country.IsoCode, nil
}

func (c *Client) lookupAPI(ip net.IP) (string, error) {
	url := fmt.Sprintf("https://ipinfo.io/%s/country?token=%s", ip.String(), c.token)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// IPInfo returns just the country code for the /country endpoint
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Response is just "US\n" format
	country := strings.TrimSpace(string(body))
	if len(country) >= 2 {
		return country[:2], nil
	}

	return "", fmt.Errorf("invalid response format")
}

// Close releases resources.
func (c *Client) Close() error {
	if c.stopUpdate != nil {
		close(c.stopUpdate)
	}
	if c.db != nil {
		return c.db.Close()
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
			c.log.Info("Checking for IPInfo database updates...")
			if err := c.updateDatabase(); err != nil {
				c.log.Error("Failed to update IPInfo database", "error", err)
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
			c.log.Debug("IPInfo database is recent, skipping update", "age", formatDuration(age))
			return nil
		}
	}

	if err := c.downloadDatabase(); err != nil {
		return err
	}

	newDB, err := geoip2.Open(c.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open updated database: %w", err)
	}

	oldDB := c.db
	c.db = newDB

	if oldDB != nil {
		oldDB.Close()
	}

	c.log.Info("IPInfo database updated and reloaded successfully")
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
