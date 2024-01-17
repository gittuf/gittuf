// SPDX-License-Identifier: Apache-2.0

package display

import (
	"errors"
	"io"
	"os"
	"os/exec"
)

func Display(defaultOutput, errorOutput io.Writer, contents []byte, page bool) error {
	if !page {
		if _, outErr := defaultOutput.Write(contents); outErr != nil {
			if _, errErr := errorOutput.Write([]byte(outErr.Error())); errErr != nil {
				return errors.Join(errErr, outErr)
			}
		}
		return nil
	}

	pagerName := os.Getenv("PAGER")
	if pagerName == "" {
		pagerName = "less" // default to less if PAGER is not set
	}

	pager := exec.Command(pagerName)

	stdInWriter, err := pager.StdinPipe()
	if err != nil {
		return err
	}

	pager.Stdout = defaultOutput
	pager.Stderr = errorOutput

	if err = pager.Start(); err != nil {
		return err
	}

	if _, err := stdInWriter.Write(contents); err != nil {
		return err
	}
	if err := stdInWriter.Close(); err != nil {
		return err
	}

	return pager.Wait()
}
