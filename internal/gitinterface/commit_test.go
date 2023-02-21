package gitinterface

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

func TestCreateCommitObject(t *testing.T) {
	gitConfig := &config.Config{
		Raw: &format.Config{
			Sections: format.Sections{
				&format.Section{
					Name: "user",
					Options: format.Options{
						&format.Option{
							Key:   "name",
							Value: "Jane Doe",
						},
						&format.Option{
							Key:   "email",
							Value: "jane.doe@example.com",
						},
					},
				},
			},
		},
	}

	clock := clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
	commit := createCommitObject(gitConfig, plumbing.ZeroHash, plumbing.ZeroHash, "Test commit", clock)

	enc := memory.NewStorage().NewEncodedObject()
	if err := commit.Encode(enc); err != nil {
		t.Error(err)
	}

	assert.Equal(t, plumbing.NewHash("dce09cc0f41eaa323f6949142d66ead789f40f6f"), enc.Hash())
}
