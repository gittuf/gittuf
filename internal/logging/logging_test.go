// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerWritesPlainMessages(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelDebug))

	logger.Debug("Loading Git repository...")
	logger.Info("Committed policy")

	assert.Equal(t, "Loading Git repository...\nCommitted policy\n", buf.String())
}

func TestHandlerDropsRecordsBelowMinimumLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelInfo))

	logger.Debug("invisible")

	assert.Empty(t, buf.String())
}

func TestHandlerPrefixesWarningsAndErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelWarn))

	logger.Warn("storer degraded")
	logger.Error("unable to write storer trace report", "error", errors.New("disk full"))

	assert.Equal(t, "Warning: storer degraded\nError: unable to write storer trace report (error=\"disk full\")\n", buf.String())
}

func TestHandlerGroupsAttributesInParentheses(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelInfo))

	logger.Info("committed policy", "ref", "refs/gittuf/policy", "count", 3)

	assert.Equal(t, "committed policy (ref=\"refs/gittuf/policy\" count=\"3\")\n", buf.String())
}

func TestHandlerQuotesAllValues(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelInfo))

	logger.Info("loading repository", "path", "/home/user/my repo", "empty", "")

	assert.Equal(t, "loading repository (path=\"/home/user/my repo\" empty=\"\")\n", buf.String())
}

func TestHandlerRendersWithAttributesOnEveryRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelInfo)).With("component", "rsl")

	logger.Info("first message")
	logger.Info("second message", "entry", "abc123")

	assert.Equal(t, "first message (component=\"rsl\")\nsecond message (component=\"rsl\" entry=\"abc123\")\n", buf.String())
}

func TestHandlerEscapesControlCharactersInMessages(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelInfo))

	// A crafted global rule name such as "refs\n\x1b[31m" must neither
	// forge an additional log line nor emit a terminal control sequence.
	logger.Info("Checking global rule\n\x1b[31m")

	assert.Equal(t, "Checking global rule\\n\\x1b[31m\n", buf.String())
}

func TestHandlerEscapesControlCharactersInValues(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelInfo))

	logger.Info("Checking global rule", "ruleName", "refs\n\x1b[31m")

	assert.Equal(t, "Checking global rule (ruleName=\"refs\\n\\x1b[31m\")\n", buf.String())
}

type redacted string

func (r redacted) LogValue() slog.Value {
	return slog.StringValue("<redacted>")
}

func TestHandlerResolvesLogValuerValues(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, slog.LevelInfo))

	logger.Info("Verifying signature", "key", redacted("secret"))

	assert.Equal(t, "Verifying signature (key=\"<redacted>\")\n", buf.String())
}

// sliceWriter records each Write call separately, and is deliberately not
// safe for concurrent use: concurrent writes must be serialized by the
// handler for the race detector to stay quiet.
type sliceWriter struct {
	writes []string
}

func (s *sliceWriter) Write(p []byte) (int, error) {
	s.writes = append(s.writes, string(p))
	return len(p), nil
}

func TestHandlerSerializesConcurrentWrites(t *testing.T) {
	w := &sliceWriter{}
	logger := slog.New(NewHandler(w, slog.LevelDebug))

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			logger.Info("one record per write")
		}()
	}
	wg.Wait()

	assert.Len(t, w.writes, goroutines)
	for _, write := range w.writes {
		assert.Equal(t, "one record per write\n", write)
	}
}
