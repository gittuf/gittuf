// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

type pagerGetter = func() string

var getPager pagerGetter = getPagerReal //nolint:revive

func getPagerReal() string {
	var pagerPrograms = []string{
		os.Getenv("PAGER"), // look at what the user has configured
		"less",             // default on unix-like systems
		"more",             // default on Windows
	}

	for _, bin := range pagerPrograms {
		if _, err := exec.LookPath(bin); err == nil {
			return bin
		}
	}

	return ""
}

func NewDisplayWriter(output io.Writer) io.WriteCloser {
	slog.Debug("Finding pager program...")
	pagerBin := getPager()
	if pagerBin != "" {
		slog.Debug(fmt.Sprintf("Found pager program %s", pagerBin))

		cmd := exec.Command(pagerBin)
		cmd.Stdout = output
		cmd.Stderr = os.Stderr
		return &pager{command: cmd}
	}

	slog.Debug("Pager program not found, writing to output directly...")
	switch output := output.(type) {
	// adityasaky: os.Stdout is an io.WriteCloser and we hardcode that as our
	// output medium. So, do we even need this check and noopwritecloser?
	// Possibly not, but I suggest we keep it until we can sufficiently evaluate
	// across multiple environments. noopwritecloser is handy for test writers
	// as well, so.
	case io.WriteCloser:
		return output
	default:
		return &noopwritecloser{
			writer: output,
		}
	}
}

// pager implements the io.WriteCloser while supporting writing buffered
// contents displayed using a pager program like less or more.
type pager struct {
	command     *exec.Cmd
	stdInWriter io.WriteCloser
	started     bool
}

func (p *pager) Write(contents []byte) (int, error) {
	if !p.started {
		// Load the page program's stdin pipe so we can feed it content to
		// display
		stdInWriter, err := p.command.StdinPipe()
		if err != nil {
			return -1, err
		}
		p.stdInWriter = stdInWriter

		// Start the page cmd
		if err := p.command.Start(); err != nil {
			return -1, err
		}

		p.started = true
	}

	return p.stdInWriter.Write(contents)
}

func (p *pager) Close() error {
	// Close the stdin pipe first as the cmd will wait indefinitely otherwise
	if p.stdInWriter != nil {
		if err := p.stdInWriter.Close(); err != nil {
			return err
		}
	}

	if p.started {
		if err := p.command.Wait(); err != nil {
			return err
		}
	}
	return nil
}

// noopwritecloser is a fallback to convert an io.Writer into io.WriteCloser. It
// adds a Close method which does nothing (i.e., it's a noop).
type noopwritecloser struct {
	writer io.Writer
}

func (n *noopwritecloser) Write(contents []byte) (int, error) {
	return n.writer.Write(contents)
}

func (n *noopwritecloser) Close() error {
	return nil
}
