// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dismissapproval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestDismissApproval(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--dismiss-approver", "jane.doe", "--review-ID", "123")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("invalid signer", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: "non-existent-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--dismiss-approver", "jane.doe", "--review-ID", "123")
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("success dismiss-approval", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo, err := gittuf.LoadRepository(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		fromRef := "refs/heads/main"
		testID := gitinterface.ZeroHash.String()
		reviewID := int64(123)
		approvers := []string{"jane.doe"}

		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(fromRef, testID, testID, approvers, nil)
		if err != nil {
			t.Fatal(err)
		}

		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(t.Context(), env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo.GetGitRepository())
		if err != nil {
			t.Fatal(err)
		}

		err = currentAttestations.SetGitHubPullRequestApprovalAttestation(repo.GetGitRepository(), env, "https://github.com", reviewID, tuf.GitHubAppRoleName, fromRef, testID, testID)
		if err != nil {
			t.Fatal(err)
		}

		err = currentAttestations.Commit(repo.GetGitRepository(), "Add GitHub pull request approval", true, false)
		if err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--dismiss-approver", "jane.doe", "--review-ID", "123")
		assert.NoError(t, err)
	})

	t.Run("success dismiss-approval with RSL entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo, err := gittuf.LoadRepository(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		fromRef := "refs/heads/main"
		testID := gitinterface.ZeroHash.String()
		reviewID := int64(123)
		approvers := []string{"jane.doe"}

		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(fromRef, testID, testID, approvers, nil)
		if err != nil {
			t.Fatal(err)
		}

		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(t.Context(), env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo.GetGitRepository())
		if err != nil {
			t.Fatal(err)
		}

		err = currentAttestations.SetGitHubPullRequestApprovalAttestation(repo.GetGitRepository(), env, "https://github.com", reviewID, tuf.GitHubAppRoleName, fromRef, testID, testID)
		if err != nil {
			t.Fatal(err)
		}

		err = currentAttestations.Commit(repo.GetGitRepository(), "Add GitHub pull request approval", true, false)
		if err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--dismiss-approver", "jane.doe", "--review-ID", "123")
		assert.NoError(t, err)
	})
}
