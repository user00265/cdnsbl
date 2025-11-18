// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package dbip implements a GeoIP backend using DB-IP Lite databases.
//
// This backend uses free DB-IP Country Lite databases in MaxMind MMDB format.
// No registration or API key is required. The database is downloaded automatically
// and cached locally. Updates are checked when the database is older than 30 days.
//
// Download URL: https://download.db-ip.com/free/
//
// Configuration:
//   - DATA_DIR: Directory for database files (default: "./data")
package dbip

import (
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/user00265/cdnsbl/internal/config"
	"github.com/user00265/cdnsbl/internal/logger"
)

// Client is a DB-IP database client.
type Client struct {
	db         *geoip2.Reader
	dbPath     string
	cfg        *config.Config
	stopUpdate chan struct{}
	log        *slog.Logger
}

// NewClient creates a new DB-IP client.
//
// The client will check for an existing database at DATA_DIR/dbip-country-lite.mmdb.
// If the database doesn't exist or is older than 30 days, it will be
// downloaded from DB-IP's free download site.
//
// Returns an error if the database cannot be downloaded or opened.
func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		dbPath:     filepath.Join(cfg.DataDir, "dbip-country-lite.mmdb"),
		cfg:        cfg,
		stopUpdate: make(chan struct{}),
		log:        logger.Component("dbip"),
	}

	if err := client.ensureDatabase(); err != nil {
		return nil, fmt.Errorf("failed to ensure database: %w", err)
	}

	db, err := geoip2.Open(client.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB-IP database: %w", err)
	}

	client.db = db
	client.log.Info("DB-IP database loaded", "path", client.dbPath)

	// Start background updater - DB-IP updates monthly (first day of month)
	go client.updateLoop(30 * 24 * time.Hour)

	return client, nil
}

func (c *Client) ensureDatabase() error {
	if _, err := os.Stat(c.dbPath); err == nil {
		info, _ := os.Stat(c.dbPath)
		age := time.Since(info.ModTime())
		if age < 30*24*time.Hour {
			c.log.Info("Using existing DB-IP database", "age", formatDuration(age))
			return nil
		}
		c.log.Info("DB-IP database is old, downloading new version", "age", formatDuration(age))
	}

	return c.downloadDatabase()
}

func (c *Client) downloadDatabase() error {
	c.log.Info("Downloading DB-IP Country Lite database...")

	if err := os.MkdirAll(filepath.Dir(c.dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Try current month first, then previous month as fallback
	now := time.Now()
	urls := []string{
		fmt.Sprintf("https://download.db-ip.com/free/dbip-country-lite-%d-%02d.mmdb.gz", now.Year(), now.Month()),
		fmt.Sprintf("https://download.db-ip.com/free/dbip-country-lite-%d-%02d.mmdb.gz", now.AddDate(0, -1, 0).Year(), now.AddDate(0, -1, 0).Month()),
	}

	var lastErr error
	for _, url := range urls {
		c.log.Info("Attempting to download database", "url", url)

		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			c.log.Warn("Download attempt failed", "url", url, "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("download failed with status: %s", resp.Status)
			c.log.Warn("Download attempt failed", "url", url, "status", resp.Status)
			resp.Body.Close()
			continue
		}

		// Successfully got a database, extract it
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzr.Close()

		tmpPath := c.dbPath + ".tmp"
		f, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}

		if _, err := io.Copy(f, gzr); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write database: %w", err)
		}
		f.Close()

		if err := os.Rename(tmpPath, c.dbPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to move database: %w", err)
		}

		c.log.Info("DB-IP database downloaded successfully", "url", url)
		return nil
	}

	return fmt.Errorf("failed to download database from all sources: %w", lastErr)
}

// LookupCountry returns the ISO 3166-1 alpha-2 country code for the given IP.
//
// Returns an error if the IP is not found in the database or the lookup fails.
func (c *Client) LookupCountry(ip net.IP) (string, error) {
	record, err := c.db.Country(ip)
	if err != nil {
		return "", err
	}

	return record.Country.IsoCode, nil
}

// Close closes the database connection and releases resources.
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
			c.log.Info("Checking for DB-IP database updates...")
			if err := c.updateDatabase(); err != nil {
				c.log.Error("Failed to update DB-IP database", "error", err)
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
			c.log.Debug("DB-IP database is recent, skipping update", "age", formatDuration(age))
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

	c.log.Info("DB-IP database updated and reloaded successfully")
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
