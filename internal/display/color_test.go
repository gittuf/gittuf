// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorer(t *testing.T) {
	tests := map[string]struct {
		c color
	}{
		"red":     {c: red},
		"green":   {c: green},
		"yellow":  {c: yellow},
		"blue":    {c: blue},
		"magenta": {c: magenta},
		"cyan":    {c: cyan},
		"gray":    {c: gray},
		"white":   {c: white},
	}

	testString := "gittuf"

	t.Run("colorer on", func(t *testing.T) {
		for name, test := range tests {
			coloredString := colorer(testString, test.c)
			assert.Equal(t, fmt.Sprintf("%s%s%s", test.c.Code(), testString, reset.Code()), coloredString, fmt.Sprintf("unexpected colored string sequence in test '%s'", name))
		}
	})

	t.Run("colorer off", func(t *testing.T) {
		colorer = colorerOff
		for name, test := range tests {
			coloredString := colorer(testString, test.c)
			assert.Equal(t, testString, coloredString, fmt.Sprintf("unexpected colored string sequence in test '%s'", name))
		}
	})
}
