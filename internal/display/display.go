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
	command *exec.Cmd
}

func (p *pager) Write(contents []byte) (int, error) {
	stdInWriter, err := p.command.StdinPipe()
	if err != nil {
		return -1, err
	}

	if err := p.command.Start(); err != nil {
		return -1, err
	}

	written, writeErr := stdInWriter.Write(contents)
	if writeErr != nil {
		return -1, writeErr
	}

	if err := stdInWriter.Close(); err != nil {
		return -1, err
	}

	if err := p.command.Wait(); err != nil {
		return -1, err
	}

	return written, nil
}

func (p *pager) Close() error {
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
