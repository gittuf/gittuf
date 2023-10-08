// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"time"

	"github.com/go-git/go-git/v5/config"
	"github.com/jonboulle/clockwork"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	testGitConfig = &config.Config{
		User: struct {
			Name  string
			Email string
		}{
			Name:  testName,
			Email: testEmail,
		},
	}
	testClock = clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
)
