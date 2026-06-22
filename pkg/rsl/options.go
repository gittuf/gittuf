// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import "github.com/gittuf/gittuf/pkg/gitinterface"

type GetLatestReferenceUpdaterEntryOptions struct {
	Reference string

	BeforeEntryID     gitinterface.Hash
	BeforeEntryNumber uint64

	UntilEntryID     gitinterface.Hash
	UntilEntryNumber uint64

	Unskipped bool

	NonGittuf bool

	IsReferenceEntry                bool
	IsPropagationEntryForRepository string
}

type GetLatestReferenceUpdaterEntryOption func(*GetLatestReferenceUpdaterEntryOptions)

// ForReference indicates that the reference entry returned must be for a
// specific Git reference.
func ForReference(reference string) GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.Reference = reference
	}
}

// BeforeEntryID searches for the matching reference entry before the specified
// entry ID. It cannot be used in combination with BeforeEntryNumber.
// BeforeEntryID is exclusive: the returned entry cannot be the reference entry
// that matches the specified ID.
func BeforeEntryID(entryID gitinterface.Hash) GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.BeforeEntryID = entryID
	}
}

// BeforeEntryNumber searches for the matching reference entry before the
// specified entry number. It cannot be used in combination with BeforeEntryID.
// BeforeEntryNumber is exclusive: the returned entry cannot be the reference
// entry that matches the specified number.
func BeforeEntryNumber(number uint64) GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.BeforeEntryNumber = number
	}
}

// UntilEntryID terminates the search for the desired reference entry when an
// entry with the specified ID is encountered. It cannot be used in combination
// with UntilEntryNumber. UntilEntryID is inclusive: the returned entry can be
// the entry that matches the specified ID.
func UntilEntryID(entryID gitinterface.Hash) GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.UntilEntryID = entryID
	}
}

// UntilEntryNumber terminates the search for the desired reference entry when
// an entry with the specified number is encountered. It cannot be used in
// combination with UntilEntryID. UntilEntryNumber is inclusive: the returned
// entry can be the entry that matches the specified number.
func UntilEntryNumber(number uint64) GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.UntilEntryNumber = number
	}
}

// IsUnskipped ensures that the returned reference entry has not been skipped by
// a subsequent annotation entry.
func IsUnskipped() GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.Unskipped = true
	}
}

// ForNonGittufReference ensures that the returned reference entry is not for a
// gittuf-specific reference.
func ForNonGittufReference() GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.NonGittuf = true
	}
}

// IsReferenceEntry ensures that the returned entry is a reference entry
// specifically, rather than any entry type that matches the ReferenceUpdater
// interface.
func IsReferenceEntry() GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.IsReferenceEntry = true
	}
}

// IsPropagationEntryForRepository ensures that the returned entry is a
// propagation entry for the specified upstream repository.
func IsPropagationEntryForRepository(repositoryLocation string) GetLatestReferenceUpdaterEntryOption {
	return func(o *GetLatestReferenceUpdaterEntryOptions) {
		o.IsPropagationEntryForRepository = repositoryLocation
	}
}
