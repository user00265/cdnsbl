// Copyright (c) 2025 Elisamuel Resto Donate <sam@samresto.dev>
// This project is licensed under the MIT License.

// Package logger provides structured logging using slog.
//
// The logger is configured based on the LOGLEVEL environment variable:
//   - debug: Debug level and above
//   - info: Info level and above (default)
//   - warn: Warning level and above
//   - error: Error level and above
//
// Log format: timestamp | LEVEL | [component] message
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"

	// Background colors for log levels
	bgRed    = "\033[41m\033[97m" // red background, white text
	bgYellow = "\033[43m\033[30m" // yellow background, black text
	bgBlue   = "\033[44m\033[97m" // blue background, white text
	bgCyan   = "\033[46m\033[30m" // cyan background, black text
)

var Log *slog.Logger

// Init initializes the global logger based on LOGLEVEL environment variable.
func Init() {
	level := getLogLevel()
	handler := NewCustomHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	Log = slog.New(handler)
}

// getLogLevel returns the slog.Level based on LOGLEVEL env var.
func getLogLevel() slog.Level {
	levelStr := strings.ToLower(os.Getenv("LOGLEVEL"))
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// CustomHandler implements slog.Handler with custom formatting.
type CustomHandler struct {
	out    io.Writer
	level  slog.Level
	groups []string
	attrs  []slog.Attr
}

// NewCustomHandler creates a new custom handler.
func NewCustomHandler(out io.Writer, opts *slog.HandlerOptions) *CustomHandler {
	level := slog.LevelInfo
	if opts != nil && opts.Level != nil {
		level = opts.Level.Level()
	}
	return &CustomHandler{
		out:   out,
		level: level,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *CustomHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and writes the log record.
func (h *CustomHandler) Handle(_ context.Context, r slog.Record) error {
	// Format: timestamp [LEVEL] [component] message
	buf := make([]byte, 0, 256)

	// Timestamp (ISO 8601)
	buf = append(buf, r.Time.Format("2006-01-02T15:04:05.000Z07:00")...)
	buf = append(buf, " "...)

	// Level with color - no padding
	levelStr := r.Level.String()

	// Color codes based on level
	var color string
	switch r.Level {
	case slog.LevelDebug:
		color = bgCyan // cyan background for debug
	case slog.LevelInfo:
		color = bgBlue // blue background for info
	case slog.LevelWarn:
		color = bgYellow // yellow background for warn
	case slog.LevelError:
		color = bgRed // red background for error
	default:
		color = colorReset
	}

	// Add colored level with square bracket delimiters
	buf = append(buf, color...)
	buf = append(buf, '[')
	buf = append(buf, ' ')
	buf = append(buf, levelStr...)
	buf = append(buf, ' ')
	buf = append(buf, ']')
	buf = append(buf, colorReset...)
	buf = append(buf, ' ')

	// Find component from handler attrs (set by WithAttrs) or record attrs
	component := ""
	otherAttrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())

	// Check handler attrs first (from With())
	for _, a := range h.attrs {
		if a.Key == "component" {
			component = a.Value.String()
		} else {
			otherAttrs = append(otherAttrs, a)
		}
	}

	// Then check record attrs
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "component" && component == "" {
			component = a.Value.String()
		} else if a.Key != "component" {
			otherAttrs = append(otherAttrs, a)
		}
		return true
	})

	// Component tag
	if component != "" {
		buf = append(buf, '[')
		buf = append(buf, component...)
		buf = append(buf, "] "...)
	}

	// Message
	buf = append(buf, r.Message...)

	// Additional attributes
	for _, a := range otherAttrs {
		buf = append(buf, ' ')
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
	}

	buf = append(buf, '\n')
	_, err := h.out.Write(buf)
	return err
}

// WithAttrs returns a new handler with the given attributes added.
func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &CustomHandler{
		out:    h.out,
		level:  h.level,
		groups: h.groups,
		attrs:  newAttrs,
	}
}

// WithGroup returns a new handler with the given group added.
func (h *CustomHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &CustomHandler{
		out:    h.out,
		level:  h.level,
		groups: newGroups,
		attrs:  h.attrs,
	}
}

// Component creates a logger with a component tag.
func Component(name string) *slog.Logger {
	return Log.With(slog.String("component", name))
}
