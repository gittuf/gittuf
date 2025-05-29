// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"fmt"
	"strings"

	githubopts "github.com/gittuf/gittuf/experimental/gittuf/options/github"
	gogithub "github.com/google/go-github/v61/github"
)

// GetGitHubClient creates a client to interact with a GitHub instance. If a
// base URL other than https://github.com is supplied, the client is configured
// to interact with the specified enterprise instance.
func GetGitHubClient(baseURL, githubToken string) (*gogithub.Client, error) {
	githubClient := gogithub.NewClient(nil).WithAuthToken(githubToken)

	if baseURL != githubopts.DefaultGitHubBaseURL {
		baseURL = strings.TrimSuffix(baseURL, "/")

		endpointAPI := fmt.Sprintf("%s/%s/%s/", baseURL, "api", "v3")
		endpointUpload := fmt.Sprintf("%s/%s/%s/", baseURL, "api", "uploads")

		var err error
		githubClient, err = githubClient.WithEnterpriseURLs(endpointAPI, endpointUpload)
		if err != nil {
			return nil, err
		}
	}

	return githubClient, nil
}
