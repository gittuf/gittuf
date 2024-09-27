// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import "github.com/gittuf/gittuf/internal/gitinterface"

type GetLatestReferenceEntryOptions struct {
	Reference string

	BeforeEntryID     gitinterface.Hash
	BeforeEntryNumber uint64

	UntilEntryID     gitinterface.Hash
	UntilEntryNumber uint64

	Unskipped bool

	NonGittuf bool
}

type GetLatestReferenceEntryOption func(*GetLatestReferenceEntryOptions)

// ForReference indicates that the reference entry returned must be for a
// specific Git reference.
func ForReference(reference string) GetLatestReferenceEntryOption {
	return func(o *GetLatestReferenceEntryOptions) {
		o.Reference = reference
	}
}

// BeforeEntryID searches for the matching reference entry before the specified
// entry ID. It cannot be used in combination with BeforeEntryNumber.
// BeforeEntryID is exclusive: the returned entry cannot be the reference entry
// that matches the specified ID.
func BeforeEntryID(entryID gitinterface.Hash) GetLatestReferenceEntryOption {
	return func(o *GetLatestReferenceEntryOptions) {
		o.BeforeEntryID = entryID
	}
}

// BeforeEntryNumber searches for the matching reference entry before the
// specified entry number. It cannot be used in combination with BeforeEntryID.
// BeforeEntryNumber is exclusive: the returned entry cannot be the reference
// entry that matches the specified number.
func BeforeEntryNumber(number uint64) GetLatestReferenceEntryOption {
	return func(o *GetLatestReferenceEntryOptions) {
		o.BeforeEntryNumber = number
	}
}

// UntilEntryID terminates the search for the desired reference entry when an
// entry with the specified ID is encountered. It cannot be used in combination
// with UntilEntryNumber. UntilEntryID is inclusive: the returned entry can be
// the entry that matches the specified ID.
func UntilEntryID(entryID gitinterface.Hash) GetLatestReferenceEntryOption {
	return func(o *GetLatestReferenceEntryOptions) {
		o.UntilEntryID = entryID
	}
}

// UntilEntryNumber terminates the search for the desired reference entry when
// an entry with the specified number is encountered. It cannot be used in
// combination with UntilEntryID. UntilEntryNumber is inclusive: the returned
// entry can be the entry that matches the specified number.
func UntilEntryNumber(number uint64) GetLatestReferenceEntryOption {
	return func(o *GetLatestReferenceEntryOptions) {
		o.UntilEntryNumber = number
	}
}

// IsUnskipped ensures that the returned reference entry has not been skipped by
// a subsequent annotation entry.
func IsUnskipped() GetLatestReferenceEntryOption {
	return func(o *GetLatestReferenceEntryOptions) {
		o.Unskipped = true
	}
}

// ForNonGittufReference ensures that the returned reference entry is not for a
// gittuf-specific reference.
func ForNonGittufReference() GetLatestReferenceEntryOption {
	return func(o *GetLatestReferenceEntryOptions) {
		o.NonGittuf = true
	}
}
