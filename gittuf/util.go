package gittuf

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"

	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/secure-systems-lab/go-securesystemslib/cjson"

	tufdata "github.com/theupdateframework/go-tuf/data"
	tufkeys "github.com/theupdateframework/go-tuf/pkg/keys"
	tufsign "github.com/theupdateframework/go-tuf/sign"
)

var METADATADIR = "../metadata" // FIXME: embed metadata in Git repo

func loadRoot(state *gitstore.State) (*tufdata.Root, error) {
	var role tufdata.Root

	roleBytes, err := state.GetCurrentMetadataBytes("root")
	if err != nil {
		return &tufdata.Root{}, err
	}

	var roleMb tufdata.Signed
	err = json.Unmarshal(roleBytes, &roleMb)
	if err != nil {
		return &tufdata.Root{}, err
	}

	err = json.Unmarshal(roleMb.Signed, &role)
	if err != nil {
		return &tufdata.Root{}, err
	}

	// TODO: use verifySignatures
	rootKeys, err := state.GetAllRootKeys()
	if err != nil {
		return &tufdata.Root{}, err
	}

	msg, err := cjson.EncodeCanonical(role)
	if err != nil {
		return &tufdata.Root{}, err
	}

	verifiedKeyIDs := []string{}
	for _, sig := range roleMb.Signatures {
		key := rootKeys[sig.KeyID]
		verifier, err := tufkeys.GetVerifier(&key)
		if err != nil {
			return &tufdata.Root{}, err
		}
		err = verifier.Verify(msg, sig.Signature)
		if err != nil {
			// TODO: do we fail for any sig that fails? What's the threshold
			// strategy?
			return &tufdata.Root{}, err
		}
		verifiedKeyIDs = append(verifiedKeyIDs, sig.KeyID)
	}

	if len(verifiedKeyIDs) == 0 {
		return &tufdata.Root{}, fmt.Errorf("root role verified with zero keys")
	}

	return &role, err
}

func loadTopLevelTargets(state *gitstore.State) (*tufdata.Targets, error) {
	rootRole, err := loadRoot(state)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	topLevelTargetsKeys := map[string]tufdata.PublicKey{}
	for _, k := range rootRole.Roles["targets"].KeyIDs {
		topLevelTargetsKeys[k] = *rootRole.Keys[k]
	}

	topLevelTargetsBytes, err := state.GetCurrentMetadataBytes("targets")
	if err != nil {
		return &tufdata.Targets{}, err
	}

	var s tufdata.Signed
	err = json.Unmarshal(topLevelTargetsBytes, &s)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	err = verifySignatures(&s, topLevelTargetsKeys, rootRole.Roles["targets"].Threshold)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	var topLevelTargets tufdata.Targets
	err = json.Unmarshal(s.Signed, &topLevelTargets)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	return &topLevelTargets, nil
}

func loadSpecificTargets(state *gitstore.State, roleName string, keys map[string]tufdata.PublicKey, threshold int) (*tufdata.Targets, error) {
	targetsBytes, err := state.GetCurrentMetadataBytes(roleName)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	var mb tufdata.Signed
	err = json.Unmarshal(targetsBytes, &mb)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	err = verifySignatures(&mb, keys, threshold)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	var role tufdata.Targets
	err = json.Unmarshal(mb.Signed, &role)
	return &role, err
}

func loadSpecificTargetsWithoutVerification(state *gitstore.State, roleName string) (*tufdata.Targets, error) {
	targetsBytes, err := state.GetCurrentMetadataBytes(roleName)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	var mb tufdata.Signed
	err = json.Unmarshal(targetsBytes, &mb)
	if err != nil {
		return &tufdata.Targets{}, err
	}

	var role tufdata.Targets
	err = json.Unmarshal(mb.Signed, &role)
	return &role, err
}

func verifySignatures(envelope *tufdata.Signed, keys map[string]tufdata.PublicKey, threshold int) error {
	var role interface{}
	if err := json.Unmarshal(envelope.Signed, &role); err != nil {
		return err
	}
	msg, err := cjson.EncodeCanonical(role)
	if err != nil {
		return err
	}
	verifiedKeyIDs := []string{}
	for _, sig := range envelope.Signatures {
		key := keys[sig.KeyID]
		verifier, err := tufkeys.GetVerifier(&key)
		if err != nil {
			return err
		}
		err = verifier.Verify(msg, sig.Signature)
		if err != nil {
			return err
		}
		verifiedKeyIDs = append(verifiedKeyIDs, sig.KeyID)
	}
	if len(verifiedKeyIDs) < threshold {
		// TODO: this threshold check can be circumvented with multiple signatures from the same key
		return fmt.Errorf("threshold not met")
	}
	return nil
}

func getTreeObjectForTargetState(state *gitstore.State, targets *tufdata.Targets, targetName string) (*object.Tree, error) {
	lastTrustedCommit, err := state.GetCommitObjectFromHash(
		convertTUFHashHexBytesToPlumbingHash(
			targets.Targets[targetName].Hashes["sha1"],
		),
	)
	if err != nil {
		return &object.Tree{}, err
	}
	return state.GetTreeObjectFromHash(lastTrustedCommit.TreeHash)
}

func LoadEd25519PublicKeyFromSslib(path string) (tufdata.PublicKey, error) {
	var pubKey tufdata.PublicKey
	pubKeyData, err := os.ReadFile(path)
	if err != nil {
		return tufdata.PublicKey{}, err
	}
	err = json.Unmarshal(pubKeyData, &pubKey)
	if err != nil {
		return tufdata.PublicKey{}, err
	}

	return pubKey, nil
}

func LoadEd25519PrivateKeyFromSslib(path string) (tufdata.PrivateKey, error) {
	var privKey tufdata.PrivateKey
	privKeyData, err := os.ReadFile(path)
	if err != nil {
		return tufdata.PrivateKey{}, err
	}
	err = json.Unmarshal(privKeyData, &privKey)
	if err != nil {
		return tufdata.PrivateKey{}, err
	}

	var keyValue KeyValue
	err = json.Unmarshal(privKey.Value, &keyValue)
	if err != nil {
		return tufdata.PrivateKey{}, err
	}
	/*
		Here, the assumption is that the key pair is in the securesystemslib
		format. However, the default python-sslib format does not contain the
		private and the public halves of the key in the "private" field as
		go-tuf expects. So, while a keypair can be generated using python-sslib,
		the public portion must be appended to the private portion in the JSON
		representation.
	*/
	if len(keyValue.Private) < ed25519.PrivateKeySize {
		fullPrivateValue, err := json.Marshal(KeyValue{
			Private: append(keyValue.Private, keyValue.Public...),
			Public:  keyValue.Public,
		})
		if err != nil {
			return tufdata.PrivateKey{}, err
		}
		return tufdata.PrivateKey{
			Type:       privKey.Type,
			Scheme:     privKey.Scheme,
			Algorithms: privKey.Algorithms,
			Value:      fullPrivateValue,
		}, nil
	}

	return privKey, nil
}

func GetEd25519PublicKeyFromPrivateKey(privKey *tufdata.PrivateKey) (tufdata.PublicKey, error) {
	var keyValue KeyValue

	err := json.Unmarshal(privKey.Value, &keyValue)
	if err != nil {
		return tufdata.PublicKey{}, err
	}

	newValue, err := json.Marshal(KeyValue{
		Private: []byte{},
		Public:  keyValue.Public,
	})
	if err != nil {
		return tufdata.PublicKey{}, err
	}

	return tufdata.PublicKey{
		Type:       privKey.Type,
		Scheme:     privKey.Scheme,
		Algorithms: privKey.Algorithms,
		Value:      newValue,
	}, nil

}

type KeyValue struct {
	Private []byte `json:"private,omitempty"`
	Public  []byte `json:"public,omitempty"`
}

func generateAndSignMbFromStruct(content interface{}, keys []tufdata.PrivateKey) (tufdata.Signed, error) {
	var newMb tufdata.Signed
	newJson, err := json.Marshal(content)
	if err != nil {
		return newMb, err
	}
	newMb = tufdata.Signed{
		Signed:     newJson,
		Signatures: []tufdata.Signature{},
	}
	for _, key := range keys {
		signer, err := tufkeys.GetSigner(&key)
		if err != nil {
			return newMb, err
		}
		err = tufsign.Sign(&newMb, signer)
		if err != nil {
			return newMb, err
		}
	}
	return newMb, nil
}

func ExpectedSignersForTarget(state *gitstore.State, target string) (map[string]tufdata.PublicKey, int, error) {
	topLevelTargets, err := loadTopLevelTargets(state)
	if err != nil {
		return map[string]tufdata.PublicKey{}, -1, err
	}

	if topLevelTargets.Delegations == nil {
		return map[string]tufdata.PublicKey{}, -1, fmt.Errorf("no rules found in targets")
	}

	for _, d := range topLevelTargets.Delegations.Roles {
		if d.Name == AllowRule {
			return map[string]tufdata.PublicKey{}, 0, nil
		}
		match, err := d.MatchesPath(target)
		if err != nil {
			return map[string]tufdata.PublicKey{}, -1, err
		}
		if match {
			keys := map[string]tufdata.PublicKey{}
			for _, k := range d.KeyIDs {
				keys[k] = *topLevelTargets.Delegations.Keys[k]
			}
			return keys, d.Threshold, nil
		}
	}
	return map[string]tufdata.PublicKey{}, -1, fmt.Errorf("no rule found for tagret %s", target)
}
