// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationResultRetainsLateLogs(t *testing.T) {
	for name, verifyErr := range map[string]error{"success": nil, "failure": errors.New("verification failed")} {
		t.Run(name, func(t *testing.T) {
			m := initialModel(context.Background(), &options{readOnly: true, targetRef: "policy"})
			m.verifying = true
			logs := make(chan string, 1)
			m.logCh = logs
			timer := time.AfterFunc(25*time.Millisecond, func() {
				logs <- "final verification log"
				close(logs)
			})
			defer timer.Stop()

			updated, _ := m.Update(verifyResultMsg{err: verifyErr, successMsg: "Verification complete"})
			result, ok := updated.(model)
			require.True(t, ok)
			assert.Contains(t, result.logsBuf.String(), "final verification log\n")
			assert.Contains(t, result.logViewport.View(), "final verification log")
			assert.False(t, result.verifying)
			assert.Nil(t, result.logCh)
			if verifyErr == nil {
				assert.Equal(t, "Verification complete", result.footer)
			} else {
				assert.Contains(t, result.errorMsg, verifyErr.Error())
			}
		})
	}
}

func TestVerificationResultDrainsClosedLogChannel(t *testing.T) {
	m := initialModel(context.Background(), &options{readOnly: true, targetRef: "policy"})
	m.verifying = true
	m.logCh = make(chan string, 2)
	m.logCh <- "first"
	m.logCh <- "last"
	close(m.logCh)
	updated, _ := m.Update(verifyResultMsg{successMsg: "done"})
	result := updated.(model)
	assert.Equal(t, "first\nlast\n", result.logsBuf.String())
	assert.Nil(t, result.logCh)
}
