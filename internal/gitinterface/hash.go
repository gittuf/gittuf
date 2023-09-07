package gitinterface

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
)

type HashAlg uint

const (
	SHA256HashAlg HashAlg = iota
	SHA512HashAlg
)

func (h HashAlg) String() string {
	switch h {
	case SHA256HashAlg:
		return "sha256"
	case SHA512HashAlg:
		return "sha512"
	}

	return ""
}

type Hash interface {
	String() string
	Bytes() []byte
}

var ZeroHash Hash

type SHA256Hash [sha256.Size]byte

func (s SHA256Hash) String() string {
	return hex.EncodeToString(s[:])
}

func (s SHA256Hash) Bytes() []byte {
	return s[:]
}

var SHA256ZeroHash SHA256Hash

type SHA512Hash [sha512.Size]byte

func (s SHA512Hash) String() string {
	return hex.EncodeToString(s[:])
}

func (s SHA512Hash) Bytes() []byte {
	return s[:]
}

func hashObj(t plumbing.ObjectType, contents []byte, hashAlg HashAlg) Hash {
	formatted := t.Bytes()
	formatted = append(formatted, ' ')
	formatted = append(formatted, []byte(fmt.Sprintf("%d", len(contents)))...)
	formatted = append(formatted, 0)
	formatted = append(formatted, contents...)

	return hash(formatted, hashAlg)
}

func hash(contents []byte, alg HashAlg) Hash {
	switch alg {
	case SHA256HashAlg:
		return SHA256Hash(sha256.Sum256(contents))
	case SHA512HashAlg:
		return SHA512Hash(sha512.Sum512(contents))
	}

	return ZeroHash
}
