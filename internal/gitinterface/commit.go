package gitinterface

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/adityasaky/gittuf/internal/signerverifier"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jonboulle/clockwork"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	gitsignVerifier "github.com/sigstore/gitsign/pkg/git"
	gitsignRekor "github.com/sigstore/gitsign/pkg/rekor"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
)

var (
	ErrUnableToSign               = errors.New("unable to sign Git object")
	ErrIncorrectVerificationKey   = errors.New("incorrect key provided to verify signature")
	ErrVerifyingSigstoreSignature = errors.New("unable to verify Sigstore signature")
)

// Commit creates a new commit in the repo and sets targetRef's HEAD to the
// commit.
func Commit(repo *git.Repository, treeHash plumbing.Hash, targetRef string, message string, sign bool) error {
	gitConfig, err := repo.ConfigScoped(config.GlobalScope)
	if err != nil {
		return err
	}

	curRef, err := repo.Reference(plumbing.ReferenceName(targetRef), true)
	if err != nil {
		return err
	}

	commit := CreateCommitObject(gitConfig, treeHash, curRef.Hash(), message, clockwork.NewRealClock())

	if sign {
		command, args, err := GetSigningCommand()
		if err != nil {
			return err
		}
		signature, err := signCommit(repo, commit, command, args)
		if err != nil {
			return err
		}
		commit.PGPSignature = signature
	}

	return ApplyCommit(repo, commit, curRef)
}

// ApplyCommit writes a commit object in the repository and updates the
// specified reference to point to the commit.
func ApplyCommit(repo *git.Repository, commit *object.Commit, curRef *plumbing.Reference) error {
	commitHash, err := WriteCommit(repo, commit)
	if err != nil {
		return err
	}

	newRef := plumbing.NewHashReference(curRef.Name(), commitHash)
	return repo.Storer.CheckAndSetReference(newRef, curRef)
}

func WriteCommit(repo *git.Repository, commit *object.Commit) (plumbing.Hash, error) {
	obj := repo.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}

	return repo.Storer.SetEncodedObject(obj)
}

// VerifyCommitSignature is used to verify a cryptographic signature associated
// with commit using TUF public keys.
func VerifyCommitSignature(ctx context.Context, commit *object.Commit, key *tuf.Key) error {
	switch key.KeyType {
	case signerverifier.GPGKeyType:
		if _, err := commit.Verify(key.KeyVal.Public); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case signerverifier.FulcioKeyType:
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

		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}

		verifiedCert, err := verifier.Verify(ctx, commitContents, []byte(commit.PGPSignature), true)
		if err != nil {
			return ErrIncorrectVerificationKey
		}

		rekor, err := gitsignRekor.New(signerverifier.RekorServer)
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
			return ErrIncorrectVerificationKey
		}

		return nil
	}

	return ErrUnknownSigningMethod
}

func CreateCommitObject(gitConfig *config.Config, treeHash plumbing.Hash, parentHash plumbing.Hash, message string, clock clockwork.Clock) *object.Commit {
	author := object.Signature{
		Name:  gitConfig.User.Name,
		Email: gitConfig.User.Email,
		When:  clock.Now(),
	}

	commit := &object.Commit{
		Author:    author,
		Committer: author,
		TreeHash:  treeHash,
		Message:   message,
	}
	if !parentHash.IsZero() {
		commit.ParentHashes = []plumbing.Hash{parentHash}
	}

	return commit
}

func signCommit(repo *git.Repository, commit *object.Commit, signingCommand string, signingArgs []string) (string, error) {
	commitContents, err := getCommitBytesWithoutSignature(commit)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(signingCommand, signingArgs...)

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

	if _, err := stdInWriter.Write(commitContents); err != nil {
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

func getCommitBytesWithoutSignature(commit *object.Commit) ([]byte, error) {
	commitEncoded := memory.NewStorage().NewEncodedObject()
	if err := commit.EncodeWithoutSignature(commitEncoded); err != nil {
		return nil, err
	}
	r, err := commitEncoded.Reader()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}
