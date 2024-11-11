// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

const DefaultGitHubBaseURL = "https://github.com"

type Options struct {
	GitHubToken   string
	GitHubBaseURL string
}

var DefaultOptions = &Options{
	GitHubBaseURL: DefaultGitHubBaseURL,
}

type Option func(o *Options)

// WithGitHubToken can be used to specify an authentication token to use the
// GitHub API.
func WithGitHubToken(token string) Option {
	return func(o *Options) {
		o.GitHubToken = token
	}
}

// WithGitHubBaseURL can be used to specify a custom GitHub instance, such as an
// on-premises GitHub Enterprise Server.
func WithGitHubBaseURL(baseURL string) Option {
	return func(o *Options) {
		o.GitHubBaseURL = baseURL
	}
}
