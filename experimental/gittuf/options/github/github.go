// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"os"
)

const (
	DefaultGitHubBaseURL = "https://github.com"

	githubTokenEnvKey = "GITHUB_TOKEN" //nolint:gosec
)

// TokenSource is a lightweight interface that can be used to fetch a GitHub
// token.
type TokenSource interface {
	Token(context.Context) (string, error)
}

type Options struct {
	GitHubTokenSource TokenSource
	GitHubBaseURL     string
	CreateRSLEntry    bool
}

var DefaultOptions = &Options{
	GitHubBaseURL:     DefaultGitHubBaseURL,
	GitHubTokenSource: &TokenSourceEnvironment{},
}

type Option func(o *Options)

// WithGitHubTokenSource can be used to specify an authentication token source
// to fetch a token to use the GitHub API.
func WithGitHubTokenSource(tokenSource TokenSource) Option {
	return func(o *Options) {
		o.GitHubTokenSource = tokenSource
	}
}

// WithGitHubBaseURL can be used to specify a custom GitHub instance, such as an
// on-premises GitHub Enterprise Server.
func WithGitHubBaseURL(baseURL string) Option {
	return func(o *Options) {
		o.GitHubBaseURL = baseURL
	}
}

func WithRSLEntry() Option {
	return func(o *Options) {
		o.CreateRSLEntry = true
	}
}

// TokenSourceEnvironment reads the GitHub API token from the GITHUB_TOKEN
// environment variable. It implements the TokenSource interface.
type TokenSourceEnvironment struct{}

func (t *TokenSourceEnvironment) Token(_ context.Context) (string, error) {
	return os.Getenv(githubTokenEnvKey), nil
}
