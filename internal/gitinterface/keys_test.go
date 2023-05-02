package gitinterface

import (
	"os"
	"testing"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/go-git/go-git/v5/config"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
)

func TestGetSigningInfo(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	repo, err := common.GetRepositoryHandler()
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		c                   *config.Config
		wantedSigningMethod SigningMethod
		wantedKeyInfo       string
		wantedProgram       string
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
			wantedSigningMethod: SigningMethodX509,
			wantedKeyInfo:       "abcdef",
			wantedProgram:       "gpgsm",
		},
	}

	for name, test := range tests {
		if err := repo.SetConfig(test.c); err != nil {
			t.Error(err)
		}

		signingMethod, keyInfo, program, err := getSigningInfo(repo)
		if err != nil {
			t.Error(err)
		}

		if signingMethod != test.wantedSigningMethod {
			t.Errorf("expected %d, got %d in test %s", test.wantedSigningMethod, signingMethod, name)
		}
		if keyInfo != test.wantedKeyInfo {
			t.Errorf("expected %s, got %s in test %s", test.wantedKeyInfo, keyInfo, name)
		}
		if program != test.wantedProgram {
			t.Errorf("expected %s, got %s in test %s", test.wantedProgram, program, name)
		}
	}

	// FIXME: We have to mock the system / global configs, this test will fail
	// because it'll detect the user's configuration.
	// This config defaults to gpg, but no signing key is specified.
	// if err := repo.SetConfig(&config.Config{
	// 	Raw: &format.Config{
	// 		Sections: format.Sections{
	// 			&format.Section{
	// 				Name: "user",
	// 				Options: format.Options{
	// 					&format.Option{
	// 						Key:   "foo",
	// 						Value: "bar",
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }); err != nil {
	// 	t.Error(err)
	// }
	// _, _, _, err = GetSigningInfo(repo)
	// assert.ErrorIs(t, err, ErrSigningKeyNotSpecified)

	// FIXME: We have to mock the system / global configs, this test will fail
	// because it'll detect the user's configuration.
	// // This config is set to use some unknown signing method.
	// if err := repo.SetConfig(&config.Config{
	// 	Raw: &format.Config{
	// 		Sections: format.Sections{
	// 			&format.Section{
	// 				Name: "user",
	// 				Options: format.Options{
	// 					&format.Option{
	// 						Key:   "signingkey",
	// 						Value: "abcdef",
	// 					},
	// 				},
	// 			},
	// 			&format.Section{
	// 				Name: "gpg",
	// 				Options: format.Options{
	// 					&format.Option{
	// 						Key:   "format",
	// 						Value: "abcdef",
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }); err != nil {
	// 	t.Error(err)
	// }

	// _, _, _, err = GetSigningInfo(repo)
	// assert.ErrorIs(t, err, ErrUnknownSigningMethod)
}
