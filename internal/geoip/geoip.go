// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package geoip implements a GeoIP backend using MaxMind GeoLite2 databases.
//
// This backend requires a MaxMind license key and account ID to download
// the GeoLite2-Country database. The database is cached locally and
// automatically updated when older than 7 days.
//
// Get a free license key at: https://www.maxmind.com/en/geolite2/signup
//
// Configuration:
//   - GEOIP_LICENSE_KEY: Your MaxMind license key (required)
//   - GEOIP_ACCOUNT_ID: Your MaxMind account ID (required)
//   - DATA_DIR: Directory for database files (default: "./data")
package geoip

import (
	"archive/tar"
	"compress/gzip"
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

// Client is a MaxMind GeoIP2 database client.
type Client struct {
	db         *geoip2.Reader
	dbPath     string
	cfg        *config.Config
	stopUpdate chan struct{}
	log        *slog.Logger
}

// NewClient creates a new GeoIP client.
//
// The client will check for an existing database at DATA_DIR/GeoLite2-Country.mmdb.
// If the database doesn't exist or is older than 7 days, it will be
// downloaded from MaxMind using the provided license key.
//
// Returns an error if the database cannot be downloaded or opened.
func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		dbPath:     filepath.Join(cfg.DataDir, "GeoLite2-Country.mmdb"),
		cfg:        cfg,
		stopUpdate: make(chan struct{}),
		log:        logger.Component("geoip"),
	}

	if err := client.ensureDatabase(); err != nil {
		return nil, fmt.Errorf("failed to ensure database: %w", err)
	}

	db, err := geoip2.Open(client.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database: %w", err)
	}

	client.db = db
	client.log.Info("GeoIP database loaded", "path", client.dbPath)

	// Start background updater - MaxMind updates weekly (Tuesdays)
	go client.updateLoop(7 * 24 * time.Hour)

	return client, nil
}

func (c *Client) ensureDatabase() error {
	if _, err := os.Stat(c.dbPath); err == nil {
		info, _ := os.Stat(c.dbPath)
		age := time.Since(info.ModTime())
		if age < 7*24*time.Hour {
			c.log.Info("Using existing GeoIP database", "age", formatDuration(age))
			return nil
		}
		c.log.Info("GeoIP database is old, downloading new version", "age", formatDuration(age))
	}

	return c.downloadDatabase()
}

func (c *Client) downloadDatabase() error {
	c.log.Info("Downloading GeoLite2-Country database...")

	if err := os.MkdirAll(filepath.Dir(c.dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	url := fmt.Sprintf(
		"https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=%s&suffix=tar.gz",
		c.cfg.GeoIPLicenseKey,
	)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		if strings.HasSuffix(header.Name, "GeoLite2-Country.mmdb") {
			tmpPath := c.dbPath + ".tmp"
			f, err := os.Create(tmpPath)
			if err != nil {
				return fmt.Errorf("failed to create temp file: %w", err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("failed to write database: %w", err)
			}
			f.Close()

			if err := os.Rename(tmpPath, c.dbPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("failed to move database: %w", err)
			}

			c.log.Info("GeoIP database downloaded successfully")
			return nil
		}
	}

	return fmt.Errorf("database file not found in archive")
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
			c.log.Info("Checking for GeoIP database updates...")
			if err := c.updateDatabase(); err != nil {
				c.log.Error("Failed to update GeoIP database", "error", err)
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
		if age < 6*24*time.Hour {
			c.log.Debug("GeoIP database is recent, skipping update", "age", formatDuration(age))
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

	c.log.Info("GeoIP database updated and reloaded successfully")
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
