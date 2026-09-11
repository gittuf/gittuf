// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/gittuf/gittuf/internal/gogitstore"
	"github.com/gittuf/gittuf/pkg/gitinterface"
)

// StorerBackendEnvKey selects the backend from the environment, for scripts
// and benchmarks that cannot pass --storer.
const StorerBackendEnvKey = "GITTUF_STORER"

// StorerBackend names a Git storage backend.
type StorerBackend string

const (
	// StorerBackendBinary invokes the git binary. It is the default and the
	// only backend gittuf makes compatibility guarantees for.
	StorerBackendBinary StorerBackend = "binary"

	// StorerBackendGoGit reads objects in-process, delegating writes and
	// merges to the git binary. Experimental.
	StorerBackendGoGit StorerBackend = "go-git"
)

// ErrUnknownStorerBackend is returned for an unrecognised backend name.
var ErrUnknownStorerBackend = errors.New("unknown Git storage backend")

const experimentalStorerWarning = "the go-git storage backend is EXPERIMENTAL: results are not covered by gittuf's compatibility guarantees and may differ from the default backend; unset --storer/" + StorerBackendEnvKey + " to disable"

// ParseStorerBackend reads a backend name. An empty name selects the default.
func ParseStorerBackend(value string) (StorerBackend, error) {
	switch backend := StorerBackend(value); backend {
	case "":
		return StorerBackendBinary, nil
	case StorerBackendBinary, StorerBackendGoGit:
		return backend, nil
	default:
		return StorerBackendBinary, fmt.Errorf("%w: %s", ErrUnknownStorerBackend, value)
	}
}

// storerSelection holds the process-wide backend and tracing settings.
type storerSelection struct {
	mu      sync.Mutex
	backend StorerBackend
	isSet   bool
	warned  bool
	trace   bool

	// warn and debugf are fields so tests can capture their output.
	warn   func(message string)
	debugf func(message string)
}

// storer is the process wide selection, set by the CLI before any command
// body runs.
var storer = &storerSelection{
	warn:   func(message string) { slog.Warn(message) },
	debugf: func(message string) { slog.Debug(message) },
}

func (s *storerSelection) set(backend StorerBackend) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.backend = backend
	s.isSet = true
}

// resolve prefers an explicit setting over the environment. A bad environment
// value is ignored rather than fatal, so it cannot block reading a repository.
func (s *storerSelection) resolve() StorerBackend {
	s.mu.Lock()
	defer s.mu.Unlock()

	backend := s.backend
	if !s.isSet {
		backend, _ = ParseStorerBackend(os.Getenv(StorerBackendEnvKey))
	}

	if backend != StorerBackendBinary && !s.warned {
		s.warned = true
		s.warn(experimentalStorerWarning)
	}

	if s.debugf != nil {
		s.debugf(fmt.Sprintf("Using '%s' Git storage backend", backend))
	}

	return backend
}

// SetStorerBackend selects the backend for subsequently loaded repositories.
func SetStorerBackend(backend StorerBackend) {
	storer.set(backend)
}

// StorerBackendInUse returns the backend this process will use.
func StorerBackendInUse() StorerBackend {
	return storer.resolve()
}

func (s *storerSelection) setTrace(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trace = enabled
}

func (s *storerSelection) tracing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trace
}

// SetStorerTrace enables per method counts and timings, read back with
// StorerTraceReport.
func SetStorerTrace(enabled bool) {
	storer.setTrace(enabled)
}

// activeTrace holds the trace for the last loaded repository.
var activeTrace struct {
	mu      sync.Mutex
	trace   *gogitstore.Trace
	repo    *gitinterface.Repository
	backend StorerBackend
}

// StorerTraceReport reports false when tracing is off or no repository was
// loaded.
func StorerTraceReport() (string, bool) {
	activeTrace.mu.Lock()
	defer activeTrace.mu.Unlock()

	if activeTrace.trace == nil || activeTrace.repo == nil {
		return "", false
	}

	return activeTrace.trace.Report(string(activeTrace.backend), activeTrace.repo.GitInvocationCount()), true
}
