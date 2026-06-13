// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	v02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/stretchr/testify/assert"
)

func TestWithInitialRootPrincipals(t *testing.T) {
	options := &LoadStateOptions{}

	principal := v02.Person{
		PersonID: "Alice",
	}

	option := WithInitialRootPrincipals([]tuf.Principal{&principal})

	option(options)

	assert.Equal(t, []tuf.Principal{&principal}, options.InitialRootPrincipals)
}

func TestBypassRSL(t *testing.T) {
	options := &LoadStateOptions{}

	option := BypassRSL()

	option(options)

	assert.True(t, options.BypassRSL)
}
