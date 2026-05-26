// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestWithMockedGitHubAPIClient(t *testing.T) {
	options := &Options{}

	assert.Panics(t, func() { _ = WithMockedGitHubAPIClient(http.DefaultClient) })

	t.Setenv("GITTUF_DEV", "1")

	option := WithMockedGitHubAPIClient(http.DefaultClient)

	option(options)

	assert.Equal(t, http.DefaultClient, options.GitHubMockedClient)
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
