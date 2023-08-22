package gitinterface

import (
	"time"

	"github.com/go-git/go-git/v5/config"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/jonboulle/clockwork"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	testGitConfig = &config.Config{
		Raw: &format.Config{
			Sections: format.Sections{
				&format.Section{
					Name: "user",
					Options: format.Options{
						&format.Option{
							Key:   "name",
							Value: testName,
						},
						&format.Option{
							Key:   "email",
							Value: testEmail,
						},
					},
				},
			},
		},
	}
	testClock = clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
)
