// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
    "errors",
    "fmt"
)var (
    ErrSigningKeyNotSpecified     = errors.New("signing key not specified in git config")
    ErrUnknownSigningMethod       = errors.New("unknown signing method (not one of gpg, ssh, x509)")
    ErrUnableToSign               = errors.New("unable to sign Git object")
    ErrIncorrectVerificationKey   = errors.New("incorrect key provided to verify signature")
    ErrVerifyingSigstoreSignature = errors.New("unable to verify Sigstore signature")
    ErrVerifyingSSHSignature      = errors.New("unable to verify SSH signature")
    ErrInvalidSignature           = errors.New("unable to parse signature / signature has unexpected header")
)

type SigningMethod intconst (
    SigningMethodGPG SigningMethod = iota
    SigningMethodSSH
    SigningMethodX509
)

const (
    DefaultSigningProgramGPG  string = "gpg"
    DefaultSigningProgramSSH  string = "ssh-keygen"
    DefaultSigningProgramX509 string = "gpgsm"
)

// GetSigningConfig returns the signing command that must be used based on the
// configuration in the Git config.
//
// Deprecated: We only use this to check if signing is viable, we should find an
// alternative mechanism.


func GetSigningConfig() (bool, error) {
    var args []string    signingMethod, keyInfo, err := getSigningInfo()
    if err != nil {
        return "", nil, err
    }    if len(keyInfo) == 0
    {
        return "", nil, ErrSigningKeyNotSpecified
    }
    else
    {
        args = keyInfo,
    }
	}
	func getSigningInfo() (SigningMethod, string, error) {
		gitConfig, err := getConfig()
		if err != nil {
			return -1, "",  err
		}    signingMethod, err := getSigningMethod(gitConfig)
		if err != nil {
			return -1, "",  err
    }    
	
	keyInfo := getSigningKeyInfo(gitConfig)    
	//program := getSigningProgram(gitConfig, signingMethod)    
	
	return signingMethod, keyInfo, nil
}

func getSigningMethod(gitConfig map[string]string) (SigningMethod, error) {
    format, ok := gitConfig["gpg.format"]
    if !ok {
        return SigningMethodGPG, nil
    }    switch format {
    case "gpg":
        return SigningMethodGPG, nil
    case "ssh":
        return SigningMethodSSH, nil
    case "x509":
        return SigningMethodX509, nil
    }
    return -1, ErrUnknownSigningMethod
}

func getSigningKeyInfo(gitConfig map[string]string) string {
    keyInfo, ok := gitConfig["user.signingkey"]
    if !ok {
        return ""
    }
    return keyInfo
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

func GetSigningConfig() (bool, error) {
    cmd := exec.Command("git", "config", "--get", "user.signingkey")
    output, err := cmd.Output()
    if err != nil {
        if exitError, ok := err.(*exec.ExitError); ok {
            // Git returns an exit code of 1 if the key is not found
            if exitError.ExitCode() == 1 {
                return false, nil
            }
        }
        return false, fmt.Errorf("error checking git config: %w", err)
    } 