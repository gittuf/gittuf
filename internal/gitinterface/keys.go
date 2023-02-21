package gitinterface

import (
	"errors"

	"github.com/go-git/go-git/v5"
)

var (
	ErrSigningKeyNotSpecified = errors.New("signing key not specified in git config")
	ErrUnknownSigningMethod   = errors.New("unknown signing method (not one of gpg, ssh, x509)")
)

type SigningMethod int

const (
	SigningMethodGPG SigningMethod = iota
	SigningMethodSSH
	SigningMethodX509
)

const (
	DefaultSigningProgramGPG  string = "gpg"
	DefaultSigningProgramSSH  string = "ssh-keygen"
	DefaultSigningProgramX509 string = "gpgsm"
)

func GetSigningCommand(repo *git.Repository) (string, []string, error) {
	var args []string

	signingMethod, keyInfo, program, err := getSigningInfo(repo)
	if err != nil {
		return "", []string{}, err
	}

	switch signingMethod {
	case SigningMethodGPG:
		args = []string{
			"-bsau", keyInfo, // b -> detach-sign, s -> sign, a -> armor, u -> local-user
		}
	case SigningMethodSSH:
		args = []string{
			"-Y", "sign",
			"-n", "git",
			"-f", keyInfo,
		}
	case SigningMethodX509:
		args = []string{
			"-bsau", keyInfo,
		}
	default:
		return "", []string{}, ErrUnknownSigningMethod
	}

	return program, args, nil
}

func getSigningInfo(repo *git.Repository) (SigningMethod, string, string, error) {
	gitConfig, err := GetConfig()
	if err != nil {
		return -1, "", "", err
	}

	signingMethod, err := getSigningMethod(gitConfig)
	if err != nil {
		return -1, "", "", err
	}

	keyInfo, err := getSigningKeyInfo(gitConfig)
	if err != nil {
		return -1, "", "", err
	}

	program := getSigningProgram(gitConfig, signingMethod)

	return signingMethod, keyInfo, program, nil
}

func getSigningMethod(gitConfig map[string]string) (SigningMethod, error) {
	format, ok := gitConfig["gpg.format"]
	if !ok {
		return SigningMethodGPG, nil
	}

	switch format {
	case "gpg":
		return SigningMethodGPG, nil
	case "ssh":
		return SigningMethodSSH, nil
	case "x509":
		return SigningMethodX509, nil
	}
	return -1, ErrUnknownSigningMethod
}

func getSigningKeyInfo(gitConfig map[string]string) (string, error) {
	keyInfo, ok := gitConfig["user.signingkey"]
	if !ok {
		return "", ErrSigningKeyNotSpecified
	}
	return keyInfo, nil
}

func getSigningProgram(gitConfig map[string]string, signingMethod SigningMethod) string {
	switch signingMethod {
	case SigningMethodSSH:
		program, ok := gitConfig["gpg.ssh.program"]
		if ok {
			return program
		}
		return DefaultSigningProgramSSH
	case SigningMethodX509:
		program, ok := gitConfig["gpg.x509.program"]
		if ok {
			return program
		}
		return DefaultSigningProgramX509
	}

	// Default to GPG
	program, ok := gitConfig["gpg.program"]
	if ok {
		return program
	}

	return DefaultSigningProgramGPG
}
