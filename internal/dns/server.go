// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package dns implements the DNS server for country-based blacklist queries.
//
// The server handles DNS queries in the format:
//
//	<octet4>.<octet3>.<octet2>.<octet1>.<country-code>.<zone>
//
// For example, to check if 210.252.48.9 is in Japan:
//
//	9.48.252.210.jp.cbl.home.lan
//
// Response:
//   - 127.0.0.1 if the IP is in the queried country
//   - NXDOMAIN if the IP is not in the queried country
//
// Only A record queries are supported. All other query types return NOTIMPL.
package dns

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/user00265/cdnsbl/internal/backend"
	"github.com/user00265/cdnsbl/internal/config"
	"github.com/user00265/cdnsbl/internal/logger"
)

// Server is a DNS server that handles country-based blacklist queries.
type Server struct {
	cfg       *config.Config
	backend   backend.Backend
	dnsServer *dns.Server
	log       *slog.Logger
}

// NewServer creates a new DNS server instance.
//
// The server will listen on the address specified in cfg.DNSListenAddr
// and handle queries for the zone specified in cfg.DNSZone.
func NewServer(cfg *config.Config, b backend.Backend) *Server {
	s := &Server{
		cfg:     cfg,
		backend: b,
		log:     logger.Component("dns"),
	}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleQuery)

	s.dnsServer = &dns.Server{
		Addr:    cfg.DNSListenAddr,
		Net:     "udp",
		Handler: mux,
	}

	return s
}

// Start starts the DNS server in a goroutine.
//
// The server listens for UDP DNS queries on the configured address.
// Returns immediately after starting the server.
func (s *Server) Start() error {
	go func() {
		if err := s.dnsServer.ListenAndServe(); err != nil {
			s.log.Info("DNS server stopped", "error", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the DNS server.
func (s *Server) Stop() {
	if s.dnsServer != nil {
		if err := s.dnsServer.Shutdown(); err != nil {
			s.log.Error("Error shutting down DNS server", "error", err)
		}
	}
}

func (s *Server) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	if len(r.Question) == 0 {
		w.WriteMsg(m)
		return
	}

	question := r.Question[0]
	qname := strings.ToLower(question.Name)

	if question.Qtype != dns.TypeA {
		m.SetRcode(r, dns.RcodeNotImplemented)
		w.WriteMsg(m)
		return
	}

	ip, countryCode, err := s.parseQuery(qname)
	if err != nil {
		// Silently return NXDOMAIN for invalid formats (e.g., zone-only queries)
		m.SetRcode(r, dns.RcodeNameError)
		w.WriteMsg(m)
		return
	}

	actualCountry, err := s.backend.LookupCountry(ip)
	if err != nil {
		s.log.Warn("Backend lookup failed", "ip", ip.String(), "error", err)
		m.SetRcode(r, dns.RcodeNameError)
		w.WriteMsg(m)
		return
	}

	if strings.EqualFold(actualCountry, countryCode) {
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   question.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP("127.0.0.1"),
		}
		m.Answer = append(m.Answer, rr)
		s.log.Debug("Match",
			"ip", ip.String(),
			"country", actualCountry,
			"queried", countryCode)
	} else {
		m.SetRcode(r, dns.RcodeNameError)
		s.log.Debug("No match",
			"ip", ip.String(),
			"country", actualCountry,
			"queried", countryCode)
	}

	w.WriteMsg(m)
}

// parseQuery extracts the IP address and country code from a DNS query name.
//
// Expected format: <o4>.<o3>.<o2>.<o1>.<cc>.<zone>
// Where o1-o4 are the IP octets in reverse order and cc is a 2-letter country code.
//
// Returns the parsed IP address and country code, or an error if the format is invalid.
func (s *Server) parseQuery(qname string) (net.IP, string, error) {
	qname = strings.TrimSuffix(qname, ".")

	if !strings.HasSuffix(qname, "."+s.cfg.DNSZone) {
		return nil, "", fmt.Errorf("query not for our zone")
	}

	qname = strings.TrimSuffix(qname, "."+s.cfg.DNSZone)

	parts := strings.Split(qname, ".")
	if len(parts) < 5 {
		return nil, "", fmt.Errorf("invalid query format, expected: <o4>.<o3>.<o2>.<o1>.<cc>.zone")
	}

	countryCode := parts[len(parts)-1]
	if len(countryCode) != 2 {
		return nil, "", fmt.Errorf("invalid country code: %s", countryCode)
	}

	ipParts := parts[:len(parts)-1]
	if len(ipParts) != 4 {
		return nil, "", fmt.Errorf("invalid IP format")
	}

	ipStr := fmt.Sprintf("%s.%s.%s.%s",
		ipParts[3], ipParts[2], ipParts[1], ipParts[0])

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, "", fmt.Errorf("invalid IP address: %s", ipStr)
	}

	return ip, strings.ToUpper(countryCode), nil
}
