// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/rsl"
)

var ErrFailedAuthentication = errors.New("failed getting remote refs")

// handleCurl implements the helper for remotes configured to use the curl
// backend. For this transport, we invoke git-remote-http, only interjecting at
// specific points to make gittuf specific additions.
func handleCurl(ctx context.Context, repo *gittuf.Repository, remoteName, url string) (map[string]string, bool, error) {
	// Scan git-remote-gittuf stdin for commands from the parent process
	stdInScanner := &logScanner{name: "git-remote-gittuf stdin", scanner: bufio.NewScanner(os.Stdin)}
	stdInScanner.Split(splitInput)

	stdOutWriter := &logWriteCloser{name: "git-remote-gittuf stdout", writeCloser: os.Stdout}

	// We invoke git-remote-http, itself a Git remote helper
	helper := exec.Command("git-remote-http", remoteName, url)
	helper.Stderr = os.Stderr

	// We want to inspect the helper's stdout for the gittuf ref statuses
	helperStdOutPipe, err := helper.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	helperStdOut := &logReadCloser{name: "git-remote-http stdout", readCloser: helperStdOutPipe}

	// We want to interpose with the helper's stdin to push and fetch gittuf
	// specific objects and refs
	helperStdInPipe, err := helper.StdinPipe()
	if err != nil {
		return nil, false, err
	}
	helperStdIn := &logWriteCloser{name: "git-remote-http stdin", writeCloser: helperStdInPipe}

	if err := helper.Start(); err != nil {
		return nil, false, err
	}

	var (
		gittufRefsTips = map[string]string{}
		isPush         bool
	)

	for stdInScanner.Scan() {
		input := stdInScanner.Bytes()

		switch {
		case bytes.HasPrefix(input, []byte("stateless-connect")):
			/*
				stateless-connect is the new experimental way of
				communicating with the remote. It implements Git Protocol
				v2. Here, we don't do much other than recognizing that we're
				in a fetch, as this protocol doesn't support pushes yet.
			*/

			log("cmd: stateless-connect")

			// Write to git-remote-http
			if _, err := helperStdIn.Write(input); err != nil {
				return nil, false, err
			}

			// Receive the initial info sent by the service via
			// git-remote-http
			seenFlush := false
			for {
				// We wrap this in an extra loop because for
				// some reason, on Windows, git-remote-http
				// responds with a buffer that just contains
				// `\n` followed by the actual response.
				// However, the initial buffer is taken to be
				// the end of output, meaning we miss the actual
				// end of output.
				helperStdOutScanner := bufio.NewScanner(helperStdOut)
				helperStdOutScanner.Split(splitOutput)

				for helperStdOutScanner.Scan() {
					output := helperStdOutScanner.Bytes()

					if _, err := stdOutWriter.Write(output); err != nil {
						return nil, false, err
					}

					// If nothing is returned, the user has likely failed to
					// authenticate with the remote
					if len(output) == 0 {
						return nil, false, ErrFailedAuthentication
					}

					// flushPkt is used to indicate the end of
					// output
					if bytes.Equal(output, flushPkt) {
						seenFlush = true
						break
					}
				}

				if seenFlush {
					break
				}
			}

			// Read in command from parent process -> this should be
			// command=ls-refs with protocol v2
			// ls-refs is a command to upload-pack. Like list and list
			// for-push, it enumerates the refs and their states on the remote.
			// Unlike those commands, this must be passed to upload-pack.
			// Further, ls-refs must be parametrized with ref-prefixes. We add
			// refs/gittuf/ as a prefix to learn about the gittuf refs on the
			// remote during fetches.
			for stdInScanner.Scan() {
				input = stdInScanner.Bytes()

				// Add ref-prefix refs/gittuf/ to the ls-refs command before
				// flush
				if bytes.Equal(input, flushPkt) {
					log("adding ref-prefix for refs/gittuf/")
					gittufRefPrefixCommand := fmt.Sprintf("ref-prefix %s\n", gittufRefPrefix)
					if _, err := helperStdIn.Write(packetEncode(gittufRefPrefixCommand)); err != nil {
						return nil, false, err
					}
				}

				if _, err := helperStdIn.Write(input); err != nil {
					return nil, false, err
				}

				// flushPkt is used to indicate the end of input
				if bytes.Equal(input, flushPkt) {
					break
				}
			}

			// Read advertised refs from the remote
			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitPacket)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				if !bytes.Equal(output, flushPkt) && !bytes.Equal(output, endOfReadPkt) {
					refAd := string(output)
					refAd = refAd[4:] // remove pkt length prefix
					refAd = strings.TrimSpace(refAd)

					// If the gittuf ref is the very first, then there will be
					// additional information in the output after a null byte.
					// However, this is unlikely as HEAD is typically the first.
					if i := strings.IndexByte(refAd, '\x00'); i > 0 {
						refAd = refAd[:i] // drop everything from null byte onwards
					}

					refAdSplit := strings.Split(refAd, " ")
					if strings.HasPrefix(refAdSplit[1], gittufRefPrefix) {
						gittufRefsTips[refAdSplit[1]] = refAdSplit[0]
					}
				}

				// Write output to parent process
				if _, err := stdOutWriter.Write(output); err != nil {
					return nil, false, err
				}

				// endOfReadPkt indicates end of response
				// in stateless connections
				if bytes.Equal(output, endOfReadPkt) {
					break
				}
			}

			// At this point, we enter the haves / wants negotiation, which is
			// followed usually by the remote sending a packfile with the
			// requested Git objects.

			// We add the gittuf specific objects as wants. We don't have to
			// specify haves as Git automatically specifies all the objects it
			// has regardless of what refs they're reachable via.

			// Read in command from parent process -> this should be
			// command=fetch with protocol v2
			var (
				wroteGittufWantsAndHaves = false // track this in case there are multiple rounds of negotiation
				wroteWants               = false
				allWants                 = set.NewSet[string]()
				allHaves                 = set.NewSet[string]()
			)
			for stdInScanner.Scan() {
				input = stdInScanner.Bytes()

				switch {
				case bytes.Equal(input, flushPkt), bytes.Contains(input, []byte("done")):
					if !wroteGittufWantsAndHaves {
						// We only write gittuf specific haves and wants when we
						// haven't already written them. We track this because
						// in multiple rounds of negotiations, we only want to
						// write them the first time.
						log("adding gittuf wants")
						wants, haves, err := getGittufWantsAndHaves(repo, gittufRefsTips)
						if err != nil {
							wants = gittufRefsTips
						}

						for _, tip := range wants {
							if !allWants.Has(tip) {
								// indicate we
								// want the
								// gittuf obj
								wantCmd := fmt.Sprintf("want %s\n", tip)
								if _, err := helperStdIn.Write(packetEncode(wantCmd)); err != nil {
									return nil, false, err
								}
							}
						}

						for _, tip := range haves {
							if !allHaves.Has(tip) {
								// indicate we
								// have the
								// gittuf obj
								haveCmd := fmt.Sprintf("have %s\n", tip)
								if _, err := helperStdIn.Write(packetEncode(haveCmd)); err != nil {
									return nil, false, err
								}
							}
						}
						wroteGittufWantsAndHaves = true
					}

					if bytes.Equal(input, flushPkt) {
						// On a clone, we see `done` and
						// then flush. We need to write
						// our wants before done, but
						// wroteWants can't be set to
						// true until the next buffer
						// with flush is written to the
						// remote.
						wroteWants = true
					}
				case bytes.Contains(input, []byte("want")):
					idx := bytes.Index(input, []byte("want "))
					sha := string(bytes.TrimSpace(input[idx+len("want "):]))
					allWants.Add(sha)

					for ref, tip := range gittufRefsTips {
						if tip == sha {
							// Take out this ref as
							// something for us to
							// update or add wants
							// for
							log("taking out", ref, "as it matches", sha)
							delete(gittufRefsTips, ref)
						}
					}
				case bytes.Contains(input, []byte("have")):
					idx := bytes.Index(input, []byte("have "))
					sha := string(bytes.TrimSpace(input[idx+len("have "):]))
					allHaves.Add(sha)
				}

				if _, err := helperStdIn.Write(input); err != nil {
					return nil, false, err
				}

				// Read from remote if wants are done
				// We may need to scan multiple times for inputs, which is
				// why this flag is used
				if wroteWants {
					helperStdOutScanner := bufio.NewScanner(helperStdOut)
					helperStdOutScanner.Split(splitPacket)

					// TODO: check multiplexed output
					for helperStdOutScanner.Scan() {
						output := helperStdOutScanner.Bytes()

						// Send along to parent process
						if _, err := stdOutWriter.Write(output); err != nil {
							return nil, false, err
						}

						if bytes.Equal(output, endOfReadPkt) {
							// Two things are possible
							// a) The communication is done
							// b) The remote indicates another round of
							// negotiation is required
							// Instead of parsing the output to find out,
							// we let the parent process tell us
							// If the parent process has further input, more
							// negotiation is needed
							if !stdInScanner.Scan() {
								break
							}

							input = stdInScanner.Bytes()
							if len(input) == 0 {
								break
							}

							for ref, tip := range gittufRefsTips {
								wantCmd := fmt.Sprintf("want %s", tip)
								if bytes.Contains(input, []byte(wantCmd)) {
									// Take out this ref as
									// something for us to
									// update or add wants
									// for
									delete(gittufRefsTips, ref)
								}
							}

							// Having scanned already, we must write prior
							// to letting the scan continue in the outer
							// loop
							// This assumes the very first input isn't just
							// flush again...
							if _, err := helperStdIn.Write(input); err != nil {
								return nil, false, err
							}
							wroteWants = false
							break
						}
					}
				}
			}

		case bytes.HasPrefix(input, []byte("list for-push")):
			/*
				The helper has two commands, in reality: list, list
				for-push. Both of these are used to list the states of refs
				on the remote. The for-push variation just formats it in a
				way that can be used for the push comamnd later.

				We inspect this to learn we're in a push. We also use the
				output of this command, implemented by git-remote-https, to
				learn what the states of the gittuf refs are on the remote.
			*/

			log("cmd: list for-push")

			// Write it to git-remote-http
			if _, err := helperStdIn.Write(input); err != nil {
				return nil, false, err
			}

			// Read remote refs
			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitOutput)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				// If nothing is returned, the user has likely failed to
				// authenticate with the remote
				if len(output) == 0 {
					return nil, false, ErrFailedAuthentication
				}

				refAdSplit := strings.Split(strings.TrimSpace(string(output)), " ")
				if len(refAdSplit) >= 2 {
					// Inspect each one to see if it's a gittuf ref
					if strings.HasPrefix(refAdSplit[1], gittufRefPrefix) {
						gittufRefsTips[refAdSplit[1]] = refAdSplit[0]
					}
				}

				// Pass remote ref status to parent process
				if _, err := stdOutWriter.Write(output); err != nil {
					return nil, false, err
				}

				// flushPkt indicates end of message
				if bytes.Equal(output, flushPkt) {
					break
				}
			}

		case bytes.HasPrefix(input, []byte("push")): // multiline input
			log("cmd: push")

			isPush = true

			pushCommands := [][]byte{}
			for !bytes.Equal(input, []byte("\n")) {
				pushCommands = append(pushCommands, input)

				if !stdInScanner.Scan() {
					break
				}
				input = stdInScanner.Bytes()
			}

			if len(gittufRefsTips) != 0 {
				if err := repo.ReconcileLocalRSLWithRemote(ctx, remoteName, true); err != nil {
					return nil, false, err
				}
			}

			// dstRefs tracks the explicitly pushed refs so we know
			// to pass the response from the server for those refs
			// back to Git
			dstRefs := set.NewSet[string]()
			for _, pushCommand := range pushCommands {
				// TODO: maybe find another way to determine
				// whether repo is gittuf enabled
				// The remote may not have gittuf refs but the
				// local may, meaning this won't get synced
				if len(gittufRefsTips) != 0 {
					pushCommandString := string(pushCommand)
					pushCommandString = strings.TrimSpace(pushCommandString)

					refSpec := strings.TrimPrefix(pushCommandString, "push ")
					refSpecSplit := strings.Split(refSpec, ":")

					srcRef := refSpecSplit[0]
					srcRef = strings.TrimPrefix(srcRef, "+") // force push
					// TODO: during a force push, we want to also revoke prior
					// pushes

					dstRef := refSpecSplit[1]
					dstRefs.Add(dstRef)

					if !strings.HasPrefix(dstRef, gittufRefPrefix) {
						// Create RSL entries for the ref as long as it's not a
						// gittuf ref
						// A gittuf ref can pop up here when it's explicitly
						// pushed by the user

						// TODO: skipping propagation; invoke it once total instead of per ref
						if err := repo.RecordRSLEntryForReference(ctx, srcRef, true, rslopts.WithOverrideRefName(dstRef), rslopts.WithSkipCheckForDuplicateEntry(), rslopts.WithRecordLocalOnly()); err != nil {
							return nil, false, err
						}
					}
				}

				// Write push command to helper
				if _, err := helperStdIn.Write(pushCommand); err != nil {
					return nil, false, err
				}
			}

			if len(gittufRefsTips) != 0 && !dstRefs.Has(rsl.Ref) {
				// Push RSL if it hasn't been explicitly pushed
				pushCommand := fmt.Sprintf("push %s:%s\n", rsl.Ref, rsl.Ref)
				if _, err := helperStdIn.Write([]byte(pushCommand)); err != nil {
					return nil, false, err
				}
			}

			// Indicate end of push statements
			if _, err := helperStdIn.Write([]byte("\n")); err != nil {
				return nil, false, err
			}

			seenTrailingNewLine := false
			for {
				// We wrap this in an extra loop because for
				// some reason, on Windows, the trailing newline
				// indicating end of output is sent in a
				// separate buffer that's otherwise missed. If
				// we miss that newline, we hang as though the
				// push isn't complete.

				helperStdOutScanner := bufio.NewScanner(helperStdOut)
				helperStdOutScanner.Split(splitOutput)

				for helperStdOutScanner.Scan() {
					output := helperStdOutScanner.Bytes()

					outputSplit := bytes.Split(output, []byte(" "))
					// outputSplit has either two items or
					// three items. It has two when the
					// response is `ok` and potentially
					// three when the response is `error`.
					// Either way, the second item is the
					// ref in question that we want to
					// bubble back to our caller.
					if len(outputSplit) < 2 {
						// This should never happen but
						// if it does, just send it back
						// to the caller
						if _, err := stdOutWriter.Write(output); err != nil {
							return nil, false, err
						}
					} else {
						if dstRefs.Has(strings.TrimSpace(string(outputSplit[1]))) {
							// this was explicitly
							// pushed by the user
							if _, err := stdOutWriter.Write(output); err != nil {
								return nil, false, err
							}
						}
					}

					if bytes.Equal(output, []byte("\n")) {
						seenTrailingNewLine = true
						break
					}
				}

				if seenTrailingNewLine {
					break
				}
			}
		default:
			// Pass through other commands we don't want to interpose to the
			// curl helper
			if _, err := helperStdIn.Write(input); err != nil {
				return nil, false, err
			}

			// Receive the initial info sent by the service
			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitOutput)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				if _, err := stdOutWriter.Write(output); err != nil {
					return nil, false, err
				}

				// Check for end of message
				if bytes.Equal(output, flushPkt) {
					break
				}
			}
		}
	}

	if err := helperStdIn.Close(); err != nil {
		return nil, false, err
	}

	if err := helperStdOut.Close(); err != nil {
		return nil, false, err
	}

	if err := helper.Wait(); err != nil {
		return nil, false, err
	}

	return gittufRefsTips, isPush, nil
}
