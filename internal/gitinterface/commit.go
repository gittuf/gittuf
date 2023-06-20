package gitinterface

import (
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
	"github.com/jonboulle/clockwork"
)

var (
	ErrUnableToSign             = errors.New("unable to sign Git object")
	ErrIncorrectVerificationKey = errors.New("incorrect key provided to verify signature")
)

func Commit(repo *git.Repository, treeHash plumbing.Hash, targetRef string, message string, sign bool) error {
	gitConfig, err := repo.ConfigScoped(config.GlobalScope)
	if err != nil {
		return err
	}

	curRef, err := repo.Reference(plumbing.ReferenceName(targetRef), true)
	if err != nil {
		return err
	}

	commit := createCommitObject(gitConfig, treeHash, curRef.Hash(), message, clockwork.NewRealClock())

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

	obj := repo.Storer.NewEncodedObject()
	err = commit.Encode(obj)
	if err != nil {
		return err
	}
	commitHash, err := repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return err
	}

	newRef := plumbing.NewHashReference(plumbing.ReferenceName(targetRef),
		commitHash)
	return repo.Storer.CheckAndSetReference(newRef, curRef)
}

func VerifyCommitSignature(commit *object.Commit, key *tuf.Key) error {
	switch key.KeyType {
	case signerverifier.GPGKeyType:
		if _, err := commit.Verify(key.KeyVal.Public); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case signerverifier.FulcioKeyType:
		return ErrUnknownSigningMethod // TODO: implement
	}

	return ErrUnknownSigningMethod
}

func createCommitObject(gitConfig *config.Config, treeHash plumbing.Hash, parentHash plumbing.Hash, message string, clock clockwork.Clock) *object.Commit {
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
	commitEncoded := repo.Storer.NewEncodedObject()
	if err := commit.EncodeWithoutSignature(commitEncoded); err != nil {
		return "", err
	}
	r, err := commitEncoded.Reader()
	if err != nil {
		return "", err
	}
	commitContents, err := io.ReadAll(r)
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
