// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"strconv"
)

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

func splitInput(data []byte, atEOF bool) (int, []byte, error) {
	if isPacketMode {
		return splitPacket(data, atEOF)
	}

	if atEOF {
		return len(data), data, bufio.ErrFinalToken
	}

	if len(data) == 0 {
		// Request more data.
		return 0, nil, nil
	}

	if bytes.HasPrefix(data, flushPkt) {
		// We have the flushPkt that'll otherwise cause it to hang.
		// This packet isn't followed by a newline a lot of the time, so we just
		// end up requesting data perennially.
		return len(flushPkt), flushPkt, nil
	}

	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have more data to process, so we just return the current line.
		return i + 1, dropCR(data[0 : i+1]), nil
	}

	// Request more data.
	return 0, nil, nil
}

func splitOutput(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if isPacketMode {
		return splitPacket(data, atEOF)
	}

	if atEOF {
		return len(data), data, bufio.ErrFinalToken
	}

	if len(data) == 0 {
		// Request more data.
		return 0, nil, nil
	}

	if bytes.HasPrefix(data, flushPkt) {
		// We have the flushPkt that'll otherwise cause it to hang.
		// This packet isn't followed by a newline a lot of the time, so we just
		// end up requesting data perennially.
		return len(flushPkt), flushPkt, nil
	}

	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		if i == len(data)-1 {
			// This is the very last newline, we need to return ErrFinalToken to
			// not block the stdin scanner anymore.
			// This is the fundamental difference between this function and
			// splitInput, because this can block stdin.
			return len(data), dropCR(data), bufio.ErrFinalToken
		}

		// We have more data to process, so we just return the current line.
		return i + 1, dropCR(data[0 : i+1]), nil
	}

	// Request more data.
	return 0, nil, nil
}

func splitPacket(data []byte, atEOF bool) (int, []byte, error) {
	if len(data) < 4 {
		return 0, nil, nil // request more
	}

	lengthB := data[:4]
	if bytes.Equal(lengthB, flushPkt) || bytes.Equal(lengthB, delimiterPkt) || bytes.Equal(lengthB, endOfReadPkt) {
		return 4, lengthB, nil
	}

	length, err := strconv.ParseInt(string(lengthB), 16, 64)
	if err != nil {
		return -1, nil, err
	}

	l := int(length)
	if l > len(data) {
		return 0, nil, nil // request more in a new buffer
	}

	if atEOF {
		if l == len(data) {
			return l, data, bufio.ErrFinalToken
		}
	}

	return l, data[:l], nil
}
