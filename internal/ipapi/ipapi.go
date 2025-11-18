// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package ipapi implements a GeoIP backend using the IP-API.com HTTP API.
//
// This backend makes real-time HTTP requests to ip-api.com for country lookups.
// No registration or API key is required, but free usage is rate-limited to
// 45 requests per minute from the same IP address.
//
// For production use with higher volume, consider using a database backend
// (geoip or dbip) instead.
//
// API documentation: https://ip-api.com/docs
//
// Rate limits:
//   - Free: 45 requests/minute
//   - No API key required for free tier
package ipapi

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/user00265/cdnsbl/internal/config"
)

// Client is an IP-API.com HTTP client.
type Client struct {
	httpClient *http.Client
	cfg        *config.Config
}

// NewClient creates a new IP-API client.
//
// The client is configured with a 5-second timeout for HTTP requests.
// No API key or authentication is required.
func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		cfg: cfg,
	}

	return client, nil
}

// response represents the JSON response from IP-API.com.
type response struct {
	Status      string `json:"status"`      // "success" or "fail"
	Message     string `json:"message"`     // Error message if status is "fail"
	CountryCode string `json:"countryCode"` // ISO 3166-1 alpha-2 country code
	Country     string `json:"country"`     // Country name
}

// LookupCountry returns the ISO 3166-1 alpha-2 country code for the given IP.
//
// Makes an HTTP request to ip-api.com. Subject to rate limiting (45 requests/minute).
// Returns an error if the request fails, times out, or the API returns an error.
func (c *Client) LookupCountry(ip net.IP) (string, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,countryCode,country", ip.String())

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return "", fmt.Errorf("API error: %s", result.Message)
	}

	return result.CountryCode, nil
}

// Close releases resources. This is a no-op for the HTTP client.
func (c *Client) Close() error {
	return nil
}
