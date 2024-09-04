// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBranchPathMatches(t *testing.T) {
	tests := map[string]struct {
		pattern  string
		target   string
		expected bool
	}{
		"full path, matches": {
			pattern:  "foo",
			target:   "foo",
			expected: true,
		},
		"artifact in directory, matches": {
			pattern:  "foo/*",
			target:   "foo/bar",
			expected: true,
		},
		"artifact in directory, does not match": {
			pattern:  "foo/*.txt",
			target:   "foo/bar.tgz",
			expected: false,
		},
		"artifact in subdirectory, matches": {
			pattern:  "foo/*",
			target:   "foo/bar/foobar",
			expected: true,
		},
		"artifact in subdirectory with specified extension, matches": {
			pattern:  "foo/*.tgz",
			target:   "foo/bar/foobar.tgz",
			expected: true,
		},
		"pattern with single character selector, matches": {
			pattern:  "foo/?.tgz",
			target:   "foo/a.tgz",
			expected: true,
		},
		"pattern with character sequence, matches": {
			pattern:  "foo/[abc].tgz",
			target:   "foo/a.tgz",
			expected: true,
		},
		"pattern with character sequence, does not match": {
			pattern:  "foo/[abc].tgz",
			target:   "foo/x.tgz",
			expected: false,
		},
		"pattern with negative character sequence, matches": {
			pattern:  "foo/[!abc].tgz",
			target:   "foo/x.tgz",
			expected: true,
		},
		"pattern with negative character sequence, does not match": {
			pattern:  "foo/[!abc].tgz",
			target:   "foo/a.tgz",
			expected: false,
		},
		"artifact in arbitrary directory, matches": {
			pattern:  "*/*.txt",
			target:   "foo/bar/foobar.txt",
			expected: true,
		},
		"artifact with specific name in arbitrary directory, matches": {
			pattern:  "*/foobar.txt",
			target:   "foo/bar/foobar.txt",
			expected: true,
		},
		"artifact with arbitrary subdirectories, matches": {
			pattern:  "foo/*/foobar.txt",
			target:   "foo/bar/baz/foobar.txt",
			expected: true,
		},
		"artifact in arbitrary directory, does not match": {
			pattern:  "*.txt",
			target:   "foo/bar/foobar.txtfile",
			expected: false,
		},
		"arbitrary directory, does not match": {
			pattern:  "*_test",
			target:   "foo/bar_test/foobar",
			expected: false,
		},
		"no pattern": {
			pattern:  "",
			target:   "foo",
			expected: false,
		},
		"pattern with multiple consecutive wildcards, matches": {
			pattern:  "foo/*/*/*.txt",
			target:   "foo/bar/baz/qux.txt",
			expected: true,
		},
		"pattern with multiple non-consecutive wildcards, matches": {
			pattern:  "foo/*/baz/*.txt",
			target:   "foo/bar/baz/qux.txt",
			expected: true,
		},
		"pattern with gittuf git prefix, matches": {
			pattern:  "git:refs/heads/*",
			target:   "git:refs/heads/main",
			expected: true,
		},
		"pattern with gittuf file prefix for all recursive contents, matches": {
			pattern:  "file:src/signatures/*",
			target:   "file:src/signatures/rsa/rsa.go",
			expected: true,
		},
	}

	for name, test := range tests {
		branchPath := BranchPath{BranchName: test.pattern}
		got := branchPath.Matches([]string{test.target})
		assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestFilePathMatches(t *testing.T) {
	t.Run("without branch scope", func(t *testing.T) {
		tests := map[string]struct {
			pattern  string
			target   string
			expected bool
		}{
			"full path, matches": {
				pattern:  "foo",
				target:   "foo",
				expected: true,
			},
			"artifact in directory, matches": {
				pattern:  "foo/*",
				target:   "foo/bar",
				expected: true,
			},
			"artifact in directory, does not match": {
				pattern:  "foo/*.txt",
				target:   "foo/bar.tgz",
				expected: false,
			},
			"artifact in subdirectory, matches": {
				pattern:  "foo/*",
				target:   "foo/bar/foobar",
				expected: true,
			},
			"artifact in subdirectory with specified extension, matches": {
				pattern:  "foo/*.tgz",
				target:   "foo/bar/foobar.tgz",
				expected: true,
			},
			"pattern with single character selector, matches": {
				pattern:  "foo/?.tgz",
				target:   "foo/a.tgz",
				expected: true,
			},
			"pattern with character sequence, matches": {
				pattern:  "foo/[abc].tgz",
				target:   "foo/a.tgz",
				expected: true,
			},
			"pattern with character sequence, does not match": {
				pattern:  "foo/[abc].tgz",
				target:   "foo/x.tgz",
				expected: false,
			},
			"pattern with negative character sequence, matches": {
				pattern:  "foo/[!abc].tgz",
				target:   "foo/x.tgz",
				expected: true,
			},
			"pattern with negative character sequence, does not match": {
				pattern:  "foo/[!abc].tgz",
				target:   "foo/a.tgz",
				expected: false,
			},
			"artifact in arbitrary directory, matches": {
				pattern:  "*/*.txt",
				target:   "foo/bar/foobar.txt",
				expected: true,
			},
			"artifact with specific name in arbitrary directory, matches": {
				pattern:  "*/foobar.txt",
				target:   "foo/bar/foobar.txt",
				expected: true,
			},
			"artifact with arbitrary subdirectories, matches": {
				pattern:  "foo/*/foobar.txt",
				target:   "foo/bar/baz/foobar.txt",
				expected: true,
			},
			"artifact in arbitrary directory, does not match": {
				pattern:  "*.txt",
				target:   "foo/bar/foobar.txtfile",
				expected: false,
			},
			"arbitrary directory, does not match": {
				pattern:  "*_test",
				target:   "foo/bar_test/foobar",
				expected: false,
			},
			"no pattern": {
				pattern:  "",
				target:   "foo",
				expected: false,
			},
			"pattern with multiple consecutive wildcards, matches": {
				pattern:  "foo/*/*/*.txt",
				target:   "foo/bar/baz/qux.txt",
				expected: true,
			},
			"pattern with multiple non-consecutive wildcards, matches": {
				pattern:  "foo/*/baz/*.txt",
				target:   "foo/bar/baz/qux.txt",
				expected: true,
			},
			"pattern with gittuf git prefix, matches": {
				pattern:  "git:refs/heads/*",
				target:   "git:refs/heads/main",
				expected: true,
			},
			"pattern with gittuf file prefix for all recursive contents, matches": {
				pattern:  "file:src/signatures/*",
				target:   "file:src/signatures/rsa/rsa.go",
				expected: true,
			},
		}

		for name, test := range tests {
			filePath := FilePath{FilePath: test.pattern}
			got := filePath.Matches([]string{test.target})
			assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
		}
	})

	t.Run("with branch scope", func(t *testing.T) {
		tests := map[string]struct {
			pattern  string
			scope    []string
			target   []string
			expected bool
		}{
			"full path, main branch, matches": {
				pattern:  "foo",
				scope:    []string{"refs/heads/main"},
				target:   []string{"foo", "refs/heads/main"},
				expected: true,
			},
			"full path, feature branch, does not match": {
				pattern:  "foo",
				scope:    []string{"refs/heads/main"},
				target:   []string{"foo", "refs/heads/feature"},
				expected: false,
			},
			"artifact in directory, main branch, matches": {
				pattern:  "foo/*",
				scope:    []string{"refs/heads/main"},
				target:   []string{"foo/bar", "refs/heads/main"},
				expected: true,
			},
			"artifact in directory, feature branch, does not match": {
				pattern:  "foo/*",
				scope:    []string{"refs/heads/main"},
				target:   []string{"foo/bar", "refs/heads/feature"},
				expected: false,
			},
		}

		for name, test := range tests {
			filePath := FilePath{FilePath: test.pattern, BranchScope: test.scope}
			got := filePath.Matches(test.target)
			assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
		}
	})
}
