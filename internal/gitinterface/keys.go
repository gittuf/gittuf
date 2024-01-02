// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	gitsignVerifier "github.com/sigstore/gitsign/pkg/git"
	gitsignRekor "github.com/sigstore/gitsign/pkg/rekor"
	rekorSSH "github.com/sigstore/rekor/pkg/pki/ssh"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"golang.org/x/crypto/ssh"
)

var (
	ErrSigningKeyNotSpecified     = errors.New("signing key not specified in git config")
	ErrUnknownSigningMethod       = errors.New("unknown signing method (not one of gpg, ssh, x509)")
	ErrUnableToSign               = errors.New("unable to sign Git object")
	ErrIncorrectVerificationKey   = errors.New("incorrect key provided to verify signature")
	ErrVerifyingSigstoreSignature = errors.New("unable to verify Sigstore signature")
	ErrVerifyingSSHSignature      = errors.New("unable to verify SSH signature")
	ErrInvalidSignature           = errors.New("unable to parse signature / signature has unexpected header")
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

const (
	magicHeaderSSHSignature string = "SSHSIG"
	pemTypeSSHSignature     string = "SSH SIGNATURE"
	namespaceSSHSignature   string = "git"
)

func GetSigningCommand() (string, []string, error) {
	var args []string

	signingMethod, keyInfo, program, err := getSigningInfo()
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

func getSigningInfo() (SigningMethod, string, string, error) {
	gitConfig, err := getConfig()
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

// signGitObject signs a Git commit or tag using the user's configured Git
// config.
func signGitObject(contents []byte) (string, error) {
	command, args, err := GetSigningCommand()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(command, args...)

	stdInWriter, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	stdOutReader, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer stdOutReader.Close()

	stdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	defer stdErrReader.Close()

	if err = cmd.Start(); err != nil {
		return "", err
	}

	if _, err := stdInWriter.Write(contents); err != nil {
		return "", err
	}
	if err := stdInWriter.Close(); err != nil {
		return "", err
	}

	sig, err := io.ReadAll(stdOutReader)
	if err != nil {
		return "", err
	}

	e, err := io.ReadAll(stdErrReader)
	if err != nil {
		return "", err
	}

	if len(e) > 0 {
		fmt.Fprint(os.Stderr, string(e))
	}

	if err = cmd.Wait(); err != nil {
		return "", err
	}

	if len(sig) == 0 {
		return "", ErrUnableToSign
	}

	return string(sig), nil
}

// verifyGitsignSignature handles the Sigstore-specific workflow involved in
// verifying commit or tag signatures issued by gitsign.
func verifyGitsignSignature(ctx context.Context, key *tuf.Key, data, signature []byte) error {
	root, err := fulcioroots.Get()
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}
	intermediate, err := fulcioroots.GetIntermediates()
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	verifier, err := gitsignVerifier.NewCertVerifier(
		gitsignVerifier.WithRootPool(root),
		gitsignVerifier.WithIntermediatePool(intermediate),
	)
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	verifiedCert, err := verifier.Verify(ctx, data, signature, true)
	if err != nil {
		return ErrIncorrectVerificationKey
	}

	rekor, err := gitsignRekor.NewWithOptions(ctx, signerverifier.RekorServer)
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	ctPub, err := cosign.GetCTLogPubs(ctx)
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	checkOpts := &cosign.CheckOpts{
		RekorClient:       rekor.Rekor,
		RootCerts:         root,
		IntermediateCerts: intermediate,
		CTLogPubKeys:      ctPub,
		RekorPubKeys:      rekor.PublicKeys(),
		Identities: []cosign.Identity{{
			Issuer:  key.KeyVal.Issuer,
			Subject: key.KeyVal.Identity,
		}},
	}

	if _, err := cosign.ValidateAndUnpackCert(verifiedCert, checkOpts); err != nil {
		return errors.Join(ErrIncorrectVerificationKey, err)
	}

	return nil
}

// verifySSHKeySignature verifies Git signatures issued by SSH keys.
func verifySSHKeySignature(key *tuf.Key, data, signature []byte) error {
	verifier, err := signerverifier.NewSignerVerifierFromTUFKey(key)
	if err != nil {
		return errors.Join(ErrVerifyingSSHSignature, err)
	}

	publicKey, err := ssh.NewPublicKey(verifier.Public())
	if err != nil {
		return errors.Join(ErrVerifyingSSHSignature, err)
	}

	sshSignature, err := decodeSSHSignature(signature)
	if err != nil {
		return errors.Join(ErrVerifyingSSHSignature, err)
	}

	if err := publicKey.Verify(data, sshSignature); err != nil {
		return errors.Join(ErrIncorrectVerificationKey, err)
	}

	return nil
}

// decodeSSHSignature unpacks the PEM encoded SSH signature into its components.
// It extracts the signature bytes and returns an ssh.Signature object. This
// helper is inspired by the SSH signature decode routine in Rekor, with
// modifications for the git namespace.
func decodeSSHSignature(signatureBytes []byte) (*ssh.Signature, error) {
	block, _ := pem.Decode(signatureBytes)
	if block == nil {
		return nil, ErrInvalidSignature
	}

	if block.Type != pemTypeSSHSignature {
		return nil, ErrInvalidSignature
	}

	wrappedSig := &rekorSSH.WrappedSig{}
	if err := ssh.Unmarshal(block.Bytes, wrappedSig); err != nil {
		return nil, err
	}

	if wrappedSig.Version != 1 {
		return nil, ErrInvalidSignature
	}

	if string(wrappedSig.MagicHeader[:]) != magicHeaderSSHSignature {
		return nil, ErrInvalidSignature
	}

	if wrappedSig.Namespace != namespaceSSHSignature {
		return nil, ErrInvalidSignature
	}

	sshSig := &ssh.Signature{}
	if err := ssh.Unmarshal([]byte(wrappedSig.Signature), sshSig); err != nil {
		return nil, err
	}

	return sshSig, nil
}
