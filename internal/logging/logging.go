// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package logging provides a slog handler for human readable CLI output. It
// renders each record as a plain message, unlike slog's TextHandler whose
// time=<timestamp> level=DEBUG msg="..." format is aimed at machine
// consumption.
package logging

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type handler struct {
	// mu serializes writes to w. slog permits concurrent Handle calls, and
	// it is shared by every handler derived from this one so that they all
	// synchronize against the same writer.
	mu       *sync.Mutex
	w        io.Writer
	minLevel slog.Level
	// prefix carries attributes added by WithAttrs so they are rendered
	// with every record.
	prefix string
}

// NewHandler returns a slog.Handler that writes one plain line per record:
// the message, followed by any attributes grouped in parentheses, with each
// value quoted. Control characters are escaped in messages and values so
// that repository-controlled content cannot forge additional log lines or
// emit terminal control sequences. Warn and Error records are prefixed so
// that they stand out from progress lines.
func NewHandler(w io.Writer, level slog.Level) slog.Handler {
	return &handler{mu: &sync.Mutex{}, w: w, minLevel: level}
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder

	switch r.Level {
	case slog.LevelWarn:
		sb.WriteString("Warning: ")
	case slog.LevelError:
		sb.WriteString("Error: ")
	}

	sb.WriteString(escapeControl(r.Message))

	attrs := []string{}
	if h.prefix != "" {
		attrs = append(attrs, h.prefix)
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a.Key+"="+strconv.Quote(a.Value.Resolve().String()))
		return true
	})

	if len(attrs) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(attrs, " "))
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, sb.String())
	return err
}

// WithAttrs returns a handler that renders the attributes with each record.
// It writes to the same writer as h, guarded by the same mutex.
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	parts := []string{}
	if h.prefix != "" {
		parts = append(parts, h.prefix)
	}
	for _, a := range attrs {
		parts = append(parts, a.Key+"="+strconv.Quote(a.Value.Resolve().String()))
	}
	return &handler{
		mu:       h.mu,
		w:        h.w,
		minLevel: h.minLevel,
		prefix:   strings.Join(parts, " "),
	}
}

// WithGroup returns the handler unchanged: the CLI does not use grouped
// attributes, and group names are simply dropped from the rendered keys.
func (h *handler) WithGroup(_ string) slog.Handler { return h }

// escapeControl escapes control characters in s so that repository-controlled
// content in a message cannot forge additional log lines or emit terminal
// control sequences. All other characters, including printable Unicode, are
// left untouched. Attribute values do not need this treatment:
// strconv.Quote escapes control characters as part of quoting.
func escapeControl(s string) string {
	hasControl := false
	for _, r := range s {
		if unicode.IsControl(r) {
			hasControl = true
			break
		}
	}
	if !hasControl {
		return s
	}

	var sb strings.Builder
	for _, r := range s {
		if !unicode.IsControl(r) {
			sb.WriteRune(r)
			continue
		}
		// QuoteRune renders the control character as a Go escape sequence
		// such as \n or \x1b; trim the surrounding single quotes.
		quoted := strconv.QuoteRune(r)
		sb.WriteString(quoted[1 : len(quoted)-1])
	}
	return sb.String()
}
