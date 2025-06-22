// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

func NewDisplayWriter(output io.Writer) io.WriteCloser {
	slog.Debug("Finding pager program...")
	pagerProg := getPager()
	if pagerProg != nil {
		binary := pagerProg.getBinary()
		slog.Debug(fmt.Sprintf("Found pager program %s", binary))

		flags := pagerProg.getFlags()
		cmd := exec.Command(binary, flags...)
		cmd.Stdout = output
		cmd.Stderr = os.Stderr
		return &pagerWriteCloser{command: cmd}
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

// pagerGetter is a pointer to the dispatcher that selects the pager program to
// use. We use this to override the "real" pagerGetter in tests.
type pagerGetter = func() pager

var getPager pagerGetter = getPagerReal //nolint:revive

func getPagerReal() pager {
	var pagerPrograms = []pager{
		newPagerEnvVar(), // look at what the user has configured
		newPagerLess(),   // default on unix-like systems
		newPagerMore(),   // default on Windows
	}

	for _, prog := range pagerPrograms {
		if _, err := exec.LookPath(prog.getBinary()); err == nil {
			return prog
		}
	}

	return nil
}

// pager implements a generic interface for a stdout pager like less or more. We
// use this interface because some environments need coloring information to be
// explicitly enabled with the pager.
type pager interface {
	getBinary() string
	getFlags() []string
}

// pagerEnvVar inspects the user's $PAGER variable and creates a pager instance
// using the information there.
type pagerEnvVar struct {
	binary string
	flags  []string
}

func newPagerEnvVar() *pagerEnvVar {
	env := os.Getenv("PAGER") // look at what the user has configured
	env = strings.TrimSpace(env)

	if env == "" {
		return &pagerEnvVar{}
	}

	split := strings.Split(env, " ")
	return &pagerEnvVar{
		binary: split[0],
		flags:  split[1:],
	}
}

func (p *pagerEnvVar) getBinary() string {
	return p.binary
}

func (p *pagerEnvVar) getFlags() []string {
	return p.flags
}

// pagerLess implements the pager interface for `less`.
type pagerLess struct{}

func newPagerLess() *pagerLess {
	return &pagerLess{}
}

func (p *pagerLess) getBinary() string {
	return "less"
}

func (p *pagerLess) getFlags() []string {
	options := os.Getenv("LESS") // see if user already has preferred LESS flags
	if options != "" {
		flags := make([]string, 0, len(options))
		split := strings.Split(options, "")
		for _, s := range split {
			flags = append(flags, fmt.Sprintf("-%s", s)) // we prefix the "-" as this is passed to exec.Command
		}
		return flags
	}

	// These are the default flags to use with less, matches what Git sets
	return []string{
		"-F", // --quit-if-one-screen
		"-R", // --RAW-CONTROL-CHARS for coloring
		"-X", // --no-init
	}
}

// pagerMore implements the pager interface for `more`.
type pagerMore struct{}

func newPagerMore() *pagerMore {
	return &pagerMore{}
}

func (p *pagerMore) getBinary() string {
	return "more"
}

func (p *pagerMore) getFlags() []string {
	return nil
}

// pagerWriteCloser implements the io.WriteCloser while supporting writing
// buffered contents displayed using a pager program like less or more.
type pagerWriteCloser struct {
	command     *exec.Cmd
	stdInWriter io.WriteCloser
	started     bool
}

func (p *pagerWriteCloser) Write(contents []byte) (int, error) {
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

func (p *pagerWriteCloser) Close() error {
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
