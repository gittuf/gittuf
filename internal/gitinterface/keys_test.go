// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/config"
	format "github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/format/config"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
)

func TestGetSigningInfo(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		c                   *config.Config
		configFile          string
		wantedSigningMethod SigningMethod
		wantedKeyInfo       string
		wantedProgram       string
		expectedError       error
	}{
		"gpg signing method, key abcdef": {
			c: &config.Config{
				Raw: &format.Config{
					Sections: format.Sections{
						&format.Section{
							Name: "user",
							Options: format.Options{
								&format.Option{
									Key:   "signingkey",
									Value: "abcdef",
								},
							},
						},
					},
				},
			},
			configFile:          filepath.Join("test-data", "config-1"),
			wantedSigningMethod: SigningMethodGPG,
			wantedKeyInfo:       "abcdef",
			wantedProgram:       "gpg",
		},
		"ssh signing method, key abcdef": {
			c: &config.Config{
				Raw: &format.Config{
					Sections: format.Sections{
						&format.Section{
							Name: "user",
							Options: format.Options{
								&format.Option{
									Key:   "signingkey",
									Value: "abcdef",
								},
							},
						},
						&format.Section{
							Name: "gpg",
							Options: format.Options{
								&format.Option{
									Key:   "format",
									Value: "ssh",
								},
							},
						},
					},
				},
			},
			configFile:          filepath.Join("test-data", "config-2"),
			wantedSigningMethod: SigningMethodSSH,
			wantedKeyInfo:       "abcdef",
			wantedProgram:       "ssh-keygen",
		},
		"x509 signing method": {
			c: &config.Config{
				Raw: &format.Config{
					Sections: format.Sections{
						&format.Section{
							Name: "user",
							Options: format.Options{
								&format.Option{
									Key:   "signingkey",
									Value: "abcdef",
								},
							},
						},
						&format.Section{
							Name: "gpg",
							Options: format.Options{
								&format.Option{
									Key:   "format",
									Value: "x509",
								},
							},
						},
					},
				},
			},
			configFile:          filepath.Join("test-data", "config-3"),
			wantedSigningMethod: SigningMethodX509,
			wantedKeyInfo:       "abcdef",
			wantedProgram:       "gpgsm",
		},
		"unknown signing key": {
			c: &config.Config{
				Raw: &format.Config{
					Sections: format.Sections{
						&format.Section{
							Name: "user",
							Options: format.Options{
								&format.Option{
									Key:   "foo",
									Value: "bar",
								},
							},
						},
					},
				},
			},
			configFile:    filepath.Join("test-data", "config-4"),
			expectedError: ErrSigningKeyNotSpecified,
		},
		"unknown signing method": {
			c: &config.Config{
				Raw: &format.Config{
					Sections: format.Sections{
						&format.Section{
							Name: "user",
							Options: format.Options{
								&format.Option{
									Key:   "signingkey",
									Value: "abcdef",
								},
							},
						},
						&format.Section{
							Name: "gpg",
							Options: format.Options{
								&format.Option{
									Key:   "format",
									Value: "abcdef",
								},
							},
						},
					},
				},
			},
			configFile:    filepath.Join("test-data", "config-5"),
			expectedError: ErrUnknownSigningMethod,
		},
	}

	for name, test := range tests {
		if err := repo.SetConfig(test.c); err != nil {
			t.Error(err)
		}

		getGitConfigFromCommand = func() (io.Reader, error) {
			return os.Open(test.configFile)
		}

		signingMethod, keyInfo, program, err := getSigningInfo()
		if err != nil {
			if assert.ErrorIs(t, err, test.expectedError) {
				continue
			}
			t.Fatal(err)
		}

		if !assert.Equal(t, test.wantedSigningMethod, signingMethod) {
			t.Errorf("expected %d, got %d in test %s", test.wantedSigningMethod, signingMethod, name)
		}
		if !assert.Equal(t, test.wantedKeyInfo, keyInfo) {
			t.Errorf("expected %s, got %s in test %s", test.wantedKeyInfo, keyInfo, name)
		}
		if !assert.Equal(t, test.wantedProgram, program) {
			t.Errorf("expected %s, got %s in test %s", test.wantedProgram, program, name)
		}
	}
}
