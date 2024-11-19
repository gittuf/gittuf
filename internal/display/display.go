// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"io"
	"os"
	"os/exec"
)

var pagerPrograms = []string{os.Getenv("PAGER"), "less", "more"}

func NewDisplayWriter(defaultOutput io.Writer, page bool) io.WriteCloser {
	if page {
		for _, bin := range pagerPrograms {
			if _, err := exec.LookPath(bin); err != nil {
				continue
			}
			return pageWriter(bin, defaultOutput)
		}
	}

	switch output := defaultOutput.(type) {
	case io.WriteCloser:
		return output
	default:
		return &noopwriter{writer: defaultOutput}
	}
}

func pageWriter(pagerBin string, defaultOutput io.Writer) *pager {
	cmd := exec.Command(pagerBin)
	cmd.Stdout = defaultOutput
	cmd.Stderr = os.Stderr

	return &pager{command: cmd}
}

type pager struct {
	command     *exec.Cmd
	stdInWriter io.WriteCloser
	started     bool
}

func (p *pager) Write(contents []byte) (int, error) {
	// Initialize the command and stdin pipe if not already started
	if !p.started {
		stdInWriter, err := p.command.StdinPipe()
		if err != nil {
			return -1, err
		}
		p.stdInWriter = stdInWriter

		if err := p.command.Start(); err != nil {
			return -1, err
		}

		p.started = true
	}

	// Write to the pager's stdin pipe
	written, err := p.stdInWriter.Write(contents)
	if err != nil {
		return -1, err
	}

	return written, nil
}

func (p *pager) Close() error {
	// Close the stdin writer if it's open
	if p.stdInWriter != nil {
		if err := p.stdInWriter.Close(); err != nil {
			return err
		}
	}

	// Wait for the pager command to finish
	if p.started {
		if err := p.command.Wait(); err != nil {
			return err
		}
	}

	return nil
}

type noopwriter struct {
	writer io.Writer
}

func (n *noopwriter) Write(contents []byte) (int, error) {
	return n.writer.Write(contents)
}

func (n *noopwriter) Close() error {
	return nil
}
