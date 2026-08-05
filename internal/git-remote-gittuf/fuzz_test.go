// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"testing"
)

// checkSplitContract verifies the bufio.SplitFunc contract: on success the
// advance count must stay within the buffer. bufio.Scanner panics otherwise,
// so a violation here is a real bug. advance must never be negative, even when
// returning an error, so that invariant is checked before the error path.
func checkSplitContract(t *testing.T, advance int, token []byte, err error, data []byte) {
	t.Helper()

	if advance < 0 {
		t.Fatalf("advance %d is negative", advance)
	}

	if err != nil && !errors.Is(err, bufio.ErrFinalToken) {
		return
	}

	if advance < 0 || advance > len(data) {
		t.Fatalf("advance %d out of bounds for data of length %d", advance, len(data))
	}

	if len(token) > len(data) {
		t.Fatalf("token length %d exceeds data length %d", len(token), len(data))
	}
}

func FuzzSplitInput(f *testing.F) {
	f.Add([]byte(""), false)
	f.Add([]byte("hello\n"), false)
	f.Add([]byte("hello\r\n"), false)
	f.Add([]byte("no newline"), true)
	f.Add(append([]byte{}, flushPkt...), false)

	f.Fuzz(func(t *testing.T, data []byte, atEOF bool) {
		advance, token, err := splitInput(data, atEOF)
		checkSplitContract(t, advance, token, err, data)
	})
}

func FuzzSplitOutput(f *testing.F) {
	f.Add([]byte(""), false)
	f.Add([]byte("hello\n"), false)
	f.Add([]byte("hello\r\n"), true)
	f.Add([]byte("no newline"), true)
	f.Add(append([]byte{}, flushPkt...), false)

	f.Fuzz(func(t *testing.T, data []byte, atEOF bool) {
		advance, token, err := splitOutput(data, atEOF)
		checkSplitContract(t, advance, token, err, data)
	})
}

func FuzzSplitPacket(f *testing.F) {
	f.Add([]byte(""), false)
	f.Add([]byte("0000"), false)
	f.Add([]byte("0006ok"), false)
	f.Add([]byte("00zz"), false)      // invalid hex length
	f.Add([]byte("-001"), false)      // negative length via leading sign
	f.Add([]byte("0003"), false)      // length shorter than its own header
	f.Add([]byte("ffffshort"), false) // length larger than buffer
	f.Add(append([]byte{}, flushPkt...), true)

	f.Fuzz(func(t *testing.T, data []byte, atEOF bool) {
		advance, token, err := splitPacket(data, atEOF)
		checkSplitContract(t, advance, token, err, data)
	})
}
