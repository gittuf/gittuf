// SPDX-License-Identifier: Apache-2.0

package display

import (
	"bytes"
	"os"
	"testing"
)

func TestDisplay(t *testing.T) {
	// Use 'cat' as the PAGER for this test. 'cat' will simply output the contents it receives.
	os.Setenv("PAGER", "cat")
	defer os.Unsetenv("PAGER") // Ensure we clean up the environment variable after the test

	tests := []struct {
		name            string
		page            bool
		contents        []byte
		wantOutput      string
		wantErrorOutput string
		wantErr         bool
	}{
		{
			name:            "simple output without paging",
			page:            false,
			contents:        []byte("Hello, world!"),
			wantOutput:      "Hello, world!",
			wantErrorOutput: "",
			wantErr:         false,
		},
		{
			name:            "simple test with paging",
			page:            true,
			contents:        []byte("Hello, world!"),
			wantOutput:      "Hello, world!",
			wantErrorOutput: "",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultOutput := &bytes.Buffer{}
			errorOutput := &bytes.Buffer{}

			err := Display(defaultOutput, errorOutput, tt.contents, tt.page)
			if err != nil {
				if !tt.wantErr {
					t.Fatal(err)
				}
			}

			if gotOutput := defaultOutput.String(); gotOutput != tt.wantOutput {
				t.Errorf("unexpected result with Display(), got stdout = %v, want %v", gotOutput, tt.wantOutput)
			}
			if gotErrOutput := errorOutput.String(); gotErrOutput != tt.wantErrorOutput {
				t.Errorf("unexpected result with Display(), got stderr = %v, want %v", gotErrOutput, tt.wantErrorOutput)
			}
		})
	}
}
