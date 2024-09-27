// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/repository"
	rslopts "github.com/gittuf/gittuf/internal/repository/options/rsl"
	"github.com/gittuf/gittuf/internal/rsl"
)

// handleCurl implements the helper for remotes configured to use the curl
// backend. For this transport, we invoke git-remote-http, only interjecting at
// specific points to make gittuf specific additions.
func handleCurl(repo *repository.Repository, remoteName, url string) (map[string]string, bool, error) {
	// Scan git-remote-gittuf stdin for commands from the parent process
	stdInScanner := &logScanner{name: "git-remote-gittuf stdin", scanner: bufio.NewScanner(os.Stdin)}
	stdInScanner.Split(splitInput)

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
			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitOutput)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				if _, err := os.Stdout.Write(output); err != nil {
					return nil, false, err
				}

				// flushPkt is used to indicate the end of
				// output
				if bytes.Equal(output, flushPkt) {
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
			helperStdOutScanner = bufio.NewScanner(helperStdOut)
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
				if _, err := os.Stdout.Write(output); err != nil {
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
				wroteGittufWants = false // track this in case there are multiple rounds of negotiation
				wroteWants       = false
			)
			for stdInScanner.Scan() {
				input = stdInScanner.Bytes()

				if bytes.Equal(input, flushPkt) {
					if !wroteGittufWants {
						log("adding gittuf wants")
						tips, err := getGittufWants(repo, gittufRefsTips)
						if err != nil {
							tips = gittufRefsTips
						}

						for _, tip := range tips {
							wantCmd := fmt.Sprintf("want %s\n", tip)
							if _, err := helperStdIn.Write(packetEncode(wantCmd)); err != nil {
								return nil, false, err
							}
						}
						wroteGittufWants = true
					}
					wroteWants = true
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
						if _, err := os.Stdout.Write(output); err != nil {
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

				refAdSplit := strings.Split(strings.TrimSpace(string(output)), " ")
				if len(refAdSplit) >= 2 {
					// Inspect each one to see if it's a gittuf ref
					if strings.HasPrefix(refAdSplit[1], gittufRefPrefix) {
						gittufRefsTips[refAdSplit[1]] = refAdSplit[0]
					}
				}

				// Pass remote ref status to parent process
				if _, err := os.Stdout.Write(output); err != nil {
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
			for {
				if bytes.Equal(input, []byte("\n")) {
					break
				}

				pushCommands = append(pushCommands, input)

				if !stdInScanner.Scan() {
					break
				}
				input = stdInScanner.Bytes()
			}

			if len(gittufRefsTips) != 0 {
				if err := repo.ReconcileLocalRSLWithRemote(context.TODO(), remoteName, true); err != nil {
					return nil, false, err
				}
			}

			rslPushed := false
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
					if dstRef == rsl.Ref {
						rslPushed = true
					}

					if !strings.HasPrefix(dstRef, gittufRefPrefix) {
						// Create RSL entries for the ref as long as it's not a
						// gittuf ref
						// A gittuf ref can pop up here when it's explicitly
						// pushed by the user

						if err := repo.RecordRSLEntryForReference(srcRef, true, rslopts.WithOverrideRefName(dstRef)); err != nil {
							return nil, false, err
						}
					}
				}

				// Write push command to helper
				if _, err := helperStdIn.Write(pushCommand); err != nil {
					return nil, false, err
				}
			}

			if len(gittufRefsTips) != 0 && !rslPushed {
				// Push RSL
				pushCommand := fmt.Sprintf("push %s:%s\n", rsl.Ref, rsl.Ref)
				if _, err := helperStdIn.Write([]byte(pushCommand)); err != nil {
					return nil, false, err
				}
			}

			// Indicate end of push statements
			if _, err := helperStdIn.Write([]byte("\n")); err != nil {
				return nil, false, err
			}

			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitOutput)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				if !bytes.Contains(output, []byte(gittufRefPrefix)) {
					// we do this because git (at the very
					// top level) inspects all the refs it's
					// been asked to push and tracks their
					// current status. it never does this
					// for the rsl ref, because only the
					// transport is pushing that ref. if we
					// don't filter this out, it knows
					// refs/gittuf/rsl got pushed, it knows
					// _what_ the previous rsl tip was (by
					// talking to the remote in list
					// for-push) but it doesn't actually
					// know the new tip of the rsl that was
					// pushed because this is loaded before
					// the transport is ever invoked.
					if _, err := os.Stdout.Write(output); err != nil {
						return nil, false, err
					}
				}

				// flushPkt indicates end of message
				if bytes.Equal(output, flushPkt) {
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

				if _, err := os.Stdout.Write(output); err != nil {
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
