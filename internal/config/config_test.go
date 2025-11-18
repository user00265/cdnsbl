// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		checkFunc   func(*Config) error
	}{
		{
			name: "default values",
			envVars: map[string]string{
				"GEOIP_LICENSE_KEY": "test_key",
				"GEOIP_ACCOUNT_ID":  "test_id",
			},
			expectError: false,
			checkFunc: func(cfg *Config) error {
				if cfg.Backend != "dbip" {
					t.Errorf("expected backend dbip, got %s", cfg.Backend)
				}
				if cfg.DNSListenAddr != ":5353" {
					t.Errorf("expected :5353, got %s", cfg.DNSListenAddr)
				}
				if cfg.DNSZone != "cbl.home.lan" {
					t.Errorf("expected cbl.home.lan, got %s", cfg.DNSZone)
				}
				return nil
			},
		},
		{
			name: "geoip backend without credentials",
			envVars: map[string]string{
				"BACKEND": "geoip",
			},
			expectError: true,
		},
		{
			name: "geoip backend with credentials",
			envVars: map[string]string{
				"BACKEND":           "geoip",
				"GEOIP_LICENSE_KEY": "my_key",
				"GEOIP_ACCOUNT_ID":  "my_id",
			},
			expectError: false,
		},
		{
			name: "dbip backend no credentials needed",
			envVars: map[string]string{
				"BACKEND": "dbip",
			},
			expectError: false,
		},
		{
			name: "ipapi backend no credentials needed",
			envVars: map[string]string{
				"BACKEND": "ipapi",
			},
			expectError: false,
		},
		{
			name: "custom values",
			envVars: map[string]string{
				"BACKEND":         "dbip",
				"DATA_DIR":        "/custom/data",
				"DNS_LISTEN_ADDR": ":8053",
				"DNS_ZONE":        "test.example.com",
			},
			expectError: false,
			checkFunc: func(cfg *Config) error {
				if cfg.DNSListenAddr != ":8053" {
					t.Errorf("expected :8053, got %s", cfg.DNSListenAddr)
				}
				if cfg.DNSZone != "test.example.com" {
					t.Errorf("expected test.example.com, got %s", cfg.DNSZone)
				}
				if cfg.DataDir != "/custom/data" {
					t.Errorf("expected /custom/data, got %s", cfg.DataDir)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(cfg); err != nil {
					t.Errorf("check function failed: %v", err)
				}
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR_EMPTY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			result := getEnvOrDefault(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
