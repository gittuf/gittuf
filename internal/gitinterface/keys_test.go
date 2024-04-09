// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"io"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

var (
	testConfig1 = artifacts.GitConfig1
	testConfig2 = artifacts.GitConfig2
	testConfig3 = artifacts.GitConfig3
	testConfig4 = artifacts.GitConfig4
)

func TestGetSigningInfo(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		c                   *config.Config
		configFile          []byte
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
			configFile:          testConfig1,
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
			configFile:          testConfig2,
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
			configFile:          testConfig3,
			wantedSigningMethod: SigningMethodX509,
			wantedKeyInfo:       "abcdef",
			wantedProgram:       "gpgsm",
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
			configFile:    testConfig4,
			expectedError: ErrUnknownSigningMethod,
		},
	}

	for name, test := range tests {
		if err := repo.SetConfig(test.c); err != nil {
			t.Error(err)
		}

		getGitConfigFromCommand = func() (io.Reader, error) {
			return bytes.NewReader(test.configFile), nil
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
