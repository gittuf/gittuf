package gittuf

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	InvalidRef   = -1
	GitBranchRef = iota
	GitTagRef
)

const (
	GitTargetScheme     = "git:"
	GitBranchIdentifier = "branch"
	GitTagIdentifier    = "tag"
)

func CreateGitTarget(refName string, refType int) (string, error) {
	switch refType {
	case GitBranchRef:
		return fmt.Sprintf("%s%s=%s", GitTargetScheme, GitBranchIdentifier, refName), nil
	case GitTagRef:
		return fmt.Sprintf("%s%s=%s", GitTargetScheme, GitTagIdentifier, refName), nil
	}
	return "", fmt.Errorf("unknown reference type for %s", refName)
}

func ParseGitTarget(uri string) (string, int, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", 0, err
	}
	if u.Scheme != "git" {
		return "", 0, fmt.Errorf("%s is not a Git object", uri)
	}

	if len(u.Opaque) == 0 {
		return "", 0, fmt.Errorf("invalid format for %s", uri)
	}

	split := strings.Split(u.Opaque, "=") // assuming there aren't multiple key value pairs
	if len(split) > 2 {
		return "", 0, fmt.Errorf("invalid format for %s", uri)
	}

	switch split[0] {
	case GitBranchIdentifier:
		return split[1], GitBranchRef, nil
	case GitTagIdentifier:
		return split[1], GitTagRef, nil
	}
	return "", InvalidRef, fmt.Errorf("invalid Git type %s in target %s", split[0], uri)
}

func IsValidGitTarget(uri string) bool {
	return strings.HasPrefix(uri, GitTargetScheme)
}
