// SPDX-License-Identifier: Apache-2.0

package display

import (
	"bytes"
	"os"
	"testing"
)

func TestNewDisplayWriter(t *testing.T) {
	// Use 'cat' as the PAGER for this test. 'cat' will simply output the contents it receives.
	os.Setenv("PAGER", "cat")
	defer os.Unsetenv("PAGER") // Ensure we clean up the environment variable after the test

	tests := []struct {
		name            string
		page            bool
		contents        []byte
		wantOutput      string
		wantErrorOutput string
	}{
		{
			name:            "simple output without paging",
			page:            false,
			contents:        []byte("Hello, world!"),
			wantOutput:      "Hello, world!",
			wantErrorOutput: "",
		},
		{
			name:            "simple test with paging",
			page:            true,
			contents:        []byte("Hello, world!"),
			wantOutput:      "Hello, world!",
			wantErrorOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultOutput := &bytes.Buffer{}

			writer := NewDisplayWriter(defaultOutput, tt.page)

			_, err := writer.Write(tt.contents)
			if err != nil {
				t.Fatal(err)
			}

			if gotOutput := defaultOutput.String(); gotOutput != tt.wantOutput {
				t.Errorf("unexpected result with Display(), got stdout = %v, want %v", gotOutput, tt.wantOutput)
			}
		})
	}
}
