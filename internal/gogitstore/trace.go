// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MethodStat records calls and elapsed time for a Storer method.
type MethodStat struct {
	Calls     int
	Delegated int
	Total     time.Duration
}

// Trace records Storer call counts and timings.
type Trace struct {
	mu          sync.Mutex
	methods     map[string]*MethodStat
	handleOpens int
}

// NewTrace returns a trace ready to attach to a Storer.
func NewTrace() *Trace {
	return &Trace{methods: map[string]*MethodStat{}}
}

// record logs one call. Callers defer it, so start is evaluated on entry.
func (t *Trace) record(method string, start time.Time, delegated bool) {
	elapsed := time.Since(start)

	t.mu.Lock()
	defer t.mu.Unlock()

	stat, has := t.methods[method]
	if !has {
		stat = &MethodStat{}
		t.methods[method] = stat
	}

	stat.Calls++
	stat.Total += elapsed
	if delegated {
		stat.Delegated++
	}
}

func (t *Trace) recordHandleOpen() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handleOpens++
}

// Stat returns the zero value for a method that was never called.
func (t *Trace) Stat(method string) MethodStat {
	t.mu.Lock()
	defer t.mu.Unlock()

	if stat, has := t.methods[method]; has {
		return *stat
	}
	return MethodStat{}
}

// HandleOpens returns the number of go-git handle opens.
func (t *Trace) HandleOpens() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.handleOpens
}

// Report renders the trace as a table, slowest method first. gitInvocations
// comes from gitinterface, which the Storer cannot count itself.
func (t *Trace) Report(backend string, gitInvocations uint64) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	names := make([]string, 0, len(t.methods))
	for name := range t.methods {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if t.methods[names[i]].Total != t.methods[names[j]].Total {
			return t.methods[names[i]].Total > t.methods[names[j]].Total
		}
		return names[i] < names[j]
	})

	report := &strings.Builder{}
	fmt.Fprintf(report, "storer trace (%s)\n", backend)
	fmt.Fprintf(report, "  %-24s %7s %7s %10s\n", "method", "calls", "deleg", "total")

	totals := MethodStat{}
	for _, name := range names {
		stat := t.methods[name]
		fmt.Fprintf(report, "  %-24s %7d %7d %10s\n", name, stat.Calls, stat.Delegated, stat.Total.Round(time.Microsecond))

		totals.Calls += stat.Calls
		totals.Delegated += stat.Delegated
		totals.Total += stat.Total
	}

	fmt.Fprintf(report, "  %-24s %7d %7d %10s\n", "all", totals.Calls, totals.Delegated, totals.Total.Round(time.Microsecond))
	fmt.Fprintf(report, "  git forks: %d   go-git handle opens: %d\n", gitInvocations, t.handleOpens)

	return report.String()
}
