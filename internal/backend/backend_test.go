// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

package backend

import (
	"net"
	"testing"

	"github.com/user00265/cdnsbl/internal/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		backend     string
		expectError bool
	}{
		{
			name:        "unknown backend",
			backend:     "unknown",
			expectError: true,
		},
		{
			name:        "empty backend",
			backend:     "",
			expectError: true,
		},
		{
			name:        "ipapi backend",
			backend:     "ipapi",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Backend: tt.backend,
			}

			backend, err := New(cfg)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if backend != nil {
					t.Errorf("expected nil backend on error")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if backend == nil {
				t.Errorf("expected backend but got nil")
			}
		})
	}
}

// Mock backend for testing
type mockBackend struct {
	lookupFunc func(net.IP) (string, error)
}

func (m *mockBackend) LookupCountry(ip net.IP) (string, error) {
	if m.lookupFunc != nil {
		return m.lookupFunc(ip)
	}
	return "US", nil
}

func (m *mockBackend) Close() error {
	return nil
}

func TestBackendInterface(t *testing.T) {
	var _ Backend = (*mockBackend)(nil)

	mock := &mockBackend{
		lookupFunc: func(ip net.IP) (string, error) {
			if ip.String() == "8.8.8.8" {
				return "US", nil
			}
			return "XX", nil
		},
	}

	// Test lookup
	country, err := mock.LookupCountry(net.ParseIP("8.8.8.8"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if country != "US" {
		t.Errorf("expected US, got %s", country)
	}

	// Test close
	if err := mock.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}
