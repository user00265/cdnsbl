// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

package dns

import (
	"testing"

	"github.com/user00265/cdnsbl/internal/config"
)

func TestParseQuery(t *testing.T) {
	s := &Server{
		cfg: &config.Config{
			DNSZone: "cbl.home.lan",
		},
	}

	tests := []struct {
		name        string
		query       string
		expectedIP  string
		expectedCC  string
		shouldError bool
	}{
		{
			name:        "United States IP",
			query:       "223.112.243.205.us.cbl.home.lan.",
			expectedIP:  "205.243.112.223",
			expectedCC:  "US",
			shouldError: false,
		},
		{
			name:        "Germany IP",
			query:       "140.58.58.37.de.cbl.home.lan.",
			expectedIP:  "37.58.58.140",
			expectedCC:  "DE",
			shouldError: false,
		},
		{
			name:        "Japan IP",
			query:       "9.48.252.210.jp.cbl.home.lan.",
			expectedIP:  "210.252.48.9",
			expectedCC:  "JP",
			shouldError: false,
		},
		{
			name:        "India IP",
			query:       "204.120.217.103.in.cbl.home.lan.",
			expectedIP:  "103.217.120.204",
			expectedCC:  "IN",
			shouldError: false,
		},
		{
			name:        "Russia IP",
			query:       "183.204.180.213.ru.cbl.home.lan.",
			expectedIP:  "213.180.204.183",
			expectedCC:  "RU",
			shouldError: false,
		},
		{
			name:        "invalid zone",
			query:       "9.48.252.210.jp.wrong.zone.",
			shouldError: true,
		},
		{
			name:        "too few parts",
			query:       "48.252.210.jp.cbl.home.lan.",
			shouldError: true,
		},
		{
			name:        "invalid country code too long",
			query:       "9.48.252.210.usa.cbl.home.lan.",
			shouldError: true,
		},
		{
			name:        "invalid country code too short",
			query:       "9.48.252.210.u.cbl.home.lan.",
			shouldError: true,
		},
		{
			name:        "invalid IP octet",
			query:       "999.48.252.210.jp.cbl.home.lan.",
			shouldError: true,
		},
		{
			name:        "no country code",
			query:       "9.48.252.210.cbl.home.lan.",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, cc, err := s.parseQuery(tt.query)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ip.String() != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip.String())
			}

			if cc != tt.expectedCC {
				t.Errorf("expected country code %s, got %s", tt.expectedCC, cc)
			}
		})
	}
}

func TestParseQueryWithoutTrailingDot(t *testing.T) {
	s := &Server{
		cfg: &config.Config{
			DNSZone: "cbl.home.lan",
		},
	}

	// Test without trailing dot
	ip, cc, err := s.parseQuery("9.48.252.210.jp.cbl.home.lan")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ip.String() != "210.252.48.9" {
		t.Errorf("expected 210.252.48.9, got %s", ip.String())
	}
	if cc != "JP" {
		t.Errorf("expected JP, got %s", cc)
	}
}

func TestParseQueryCaseInsensitive(t *testing.T) {
	s := &Server{
		cfg: &config.Config{
			DNSZone: "cbl.home.lan",
		},
	}

	tests := []struct {
		query      string
		expectedCC string
	}{
		{"9.48.252.210.JP.cbl.home.lan.", "JP"},
		{"9.48.252.210.jp.cbl.home.lan.", "JP"},
		{"9.48.252.210.Jp.cbl.home.lan.", "JP"},
		{"9.48.252.210.jP.cbl.home.lan.", "JP"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			_, cc, err := s.parseQuery(tt.query)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if cc != tt.expectedCC {
				t.Errorf("expected %s, got %s", tt.expectedCC, cc)
			}
		})
	}
}
