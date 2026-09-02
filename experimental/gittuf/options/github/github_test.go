// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithGitHubTokenSource(t *testing.T) {
	options := &Options{}

	tokenSource := &TokenSourceEnvironment{}
	option := WithGitHubTokenSource(tokenSource)
	option(options)

	assert.Equal(t, tokenSource, options.GitHubTokenSource)
}

func TestDefaultOptions(t *testing.T) {
	assert.Equal(t, DefaultGitHubBaseURL, DefaultOptions.GitHubBaseURL)
	assert.Equal(t, &TokenSourceEnvironment{}, DefaultOptions.GitHubTokenSource)
}

func TestWithGitHubBaseURL(t *testing.T) {
	options := &Options{}

	option := WithGitHubBaseURL("example.com")
	option(options)

	assert.Equal(t, "example.com", options.GitHubBaseURL)
}

func TestWithRSLEntry(t *testing.T) {
	options := &Options{}

	option := WithRSLEntry()
	option(options)

	assert.True(t, options.CreateRSLEntry)
}

func TestWithUseGitHubAPI(t *testing.T) {
	options := &Options{}

	option := WithUseGitHubAPI()
	option(options)

	assert.True(t, options.UseGitHubAPI)
}

func TestTokenSourceEnvironment(t *testing.T) {
	t.Run("with valid GITHUB_TOKEN env var", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-token")
		source := &TokenSourceEnvironment{}
		token, err := source.Token(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "test-token", token)
	})
	t.Run("with empty GITHUB_TOKEN env var", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		source := &TokenSourceEnvironment{}
		token, err := source.Token(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "", token)
	})
}
