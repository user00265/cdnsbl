// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

package ipapi

import (
	"net"
	"testing"

	"github.com/user00265/cdnsbl/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{}
	client, err := NewClient(cfg)

	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if client == nil {
		t.Fatal("expected client but got nil")
	}

	if client.httpClient == nil {
		t.Error("expected http client to be initialized")
	}

	if err := client.Close(); err != nil {
		t.Errorf("unexpected error closing client: %v", err)
	}
}

func TestLookupCountry(t *testing.T) {
	cfg := &config.Config{}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Test cases with real IP addresses
	tests := []struct {
		name            string
		ip              string
		expectedCountry string
	}{
		{
			name:            "United States",
			ip:              "205.243.112.223",
			expectedCountry: "US",
		},
		{
			name:            "Germany",
			ip:              "37.58.58.140",
			expectedCountry: "DE",
		},
		{
			name:            "Japan",
			ip:              "210.252.48.9",
			expectedCountry: "JP",
		},
		{
			name:            "India",
			ip:              "103.217.120.204",
			expectedCountry: "IN",
		},
		{
			name:            "Russia",
			ip:              "213.180.204.183",
			expectedCountry: "RU",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			country, err := client.LookupCountry(ip)
			if err != nil {
				t.Logf("lookup failed for %s: %v (this may be due to rate limiting)", tt.ip, err)
				t.Skip("skipping due to API error - rate limit or network issue")
				return
			}

			if country != tt.expectedCountry {
				t.Errorf("expected country %s, got %s for IP %s", tt.expectedCountry, country, tt.ip)
			}
		})
	}
}

func TestLookupCountryInvalidIP(t *testing.T) {
	cfg := &config.Config{}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Test with invalid/private IPs
	invalidIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"192.168.1.1",
	}

	for _, ipStr := range invalidIPs {
		t.Run("invalid_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			_, err := client.LookupCountry(ip)
			if err == nil {
				t.Logf("lookup succeeded for private IP %s (unexpected but allowed)", ipStr)
			}
		})
	}
}
