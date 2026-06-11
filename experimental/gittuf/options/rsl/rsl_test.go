// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithOverrideName(t *testing.T) {
	options := &RecordOptions{}

	option := WithOverrideRefName("refs/gittuf/override")

	option(options)

	assert.Equal(t, "refs/gittuf/override", options.RefNameOverride)
}

func TestWithSkipCheckForDuplicateEntry(t *testing.T) {
	options := &RecordOptions{}

	option := WithSkipCheckForDuplicateEntry()

	option(options)

	assert.True(t, options.SkipCheckForDuplicate)
}

func TestWithRecordRemote(t *testing.T) {
	options := &RecordOptions{}

	option := WithRecordRemote("origin")

	option(options)

	assert.Equal(t, "origin", options.RemoteName)
}

func TestWithRecordLocalOnly(t *testing.T) {
	options := &RecordOptions{}

	option := WithRecordLocalOnly()

	option(options)

	assert.True(t, options.LocalOnly)
}

func TestWithAnnotateRemote(t *testing.T) {
	options := &AnnotateOptions{}

	option := WithAnnotateRemote("origin")

	option(options)

	assert.Equal(t, "origin", options.RemoteName)
}

func TestWithAnnotateLocalOnly(t *testing.T) {
	options := &AnnotateOptions{}

	option := WithAnnotateLocalOnly()

	option(options)

	assert.True(t, options.LocalOnly)
}
