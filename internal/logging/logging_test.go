// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"errors"
	"log/slog"
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
