// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

// handleSSH implements the helper for remotes configured to use SSH. For this
// transport, we invoke the installed ssh binary to interact with the remote.
func handleSSH(ctx context.Context, repo *gittuf.Repository, remoteName, url string) (map[string]string, bool, error) {
	url = strings.TrimPrefix(url, "ssh://")
	url = strings.TrimPrefix(url, "git+ssh://")
	url = strings.TrimPrefix(url, "ssh+git://")

	urlSplit := strings.Split(url, ":") // 0 is the connection [user@]host, 1 is the repo
	host := urlSplit[0]
	repository := urlSplit[1]

	// Scan git-remote-gittuf stdin for commands from the parent process
	stdInScanner := &logScanner{name: "git-remote-gittuf stdin", scanner: bufio.NewScanner(os.Stdin)}
	stdInScanner.Split(splitInput)

	stdOutWriter := &logWriteCloser{name: "git-remote-gittuf stdout", writeCloser: os.Stdout}

	var (
		helperStdOut   io.ReadCloser
		helperStdIn    io.WriteCloser
		gittufRefsTips = map[string]string{}
		remoteRefTips  = map[string]string{}
	)

	for stdInScanner.Scan() {
		input := stdInScanner.Bytes()

		switch {
		case bytes.HasPrefix(input, []byte("capabilities")):
			/*
				For SSH, we have several options wrt capabilities. First, we
				could just implement fetch and push. These are v0/v1 protocols.
				The issue here is that while push is fine, fetch effectively
				fetches _all_ refs it sees on the remote via list. Additionally,
				using v2 protocol where possible seems good for efficiency
				improvements hinted at by the docs.

				The connect capability sets up a bidirectional connection with
				the server. It can handle both fetches and pushes; depending on
				what's happening, either upload-pack or receive-pack must be
				invoked on the server. This is fine for fetch operations.
				However, for push, we can tell the server to set
				refs/gittuf/<whatever> to the object. However, we do not control
				the invocation of git pack-objects --stdout. Git (which invokes
				us) invokes pack-objects separately, and routes its stdout into
				the transport's stdin to transmit the packfile bytes.

				In summary, we cannot use a combination of fetch and push, and
				we cannot use connect. What about stateless-connect?  This is
				part of the v2 protocol and can only handle fetches at the
				moment. It's marked as experimental, which is something we want
				to be wary about with new Git versions.  There may well be
				breaking changes here, given that the only intended user of this
				command is other Git tooling.

				stateless-connect is quite easy to work with to handle the fetch
				aspects. In addition, we implement the push capability. Here,
				Git tells us the refspecs that must be pushed. We are separately
				responsible for actually sending the packfile(s). So, the
				solution is that we create RSL entries for each requested ref,
				and include the gittuf objects in the packfile. Thus, we specify
				stateless-connect and push as the two capabilities supported by
				this helper.
			*/

			log("cmd: capabilities")

			if _, err := stdOutWriter.Write([]byte("stateless-connect\npush\n\n")); err != nil {
				return nil, false, err
			}

		case bytes.HasPrefix(input, []byte("stateless-connect")):
			/*
				When we see stateless-connect, right now we know this means
				a fetch is underway.

				us: ssh -o SendEnv=GIT_PROTOCOL <url> 'git-upload-pack <repo>'
				ssh:
					if v2 {
						server capabilities
					} else {
						server capabilities
						refs and their states
					}
				Assuming v2:
				us (to ssh): ls-refs // add gittuf prefix
				ssh: refs and their states
				us (to git): output of ls-refs
				git: fetch, wants, haves
				us (to ssh): fetch, wants, haves // add gittuf wants
				ssh: acks (optionally triggers another round of wants, haves)
				ssh: packfile
				us (to git): acks, packfile

				Assuming v0/v1:
				git: wants, haves // NO FETCH HERE IIRC
				us (to ssh): wants, haves // add gittuf wants
				ssh: acks, packfile
				us (to git): acks, packfile

				Notes:
					* v0/v1 of the pack protocol is only partially supported
					  here.
					* Once the service is invoked, all messages are wrapped
					  in the packet-line format.
					* In the v0/v1 format, each line is packet encoded, and
					  the entire message is in turn packet encoded for
					  wants/haves.
					* The flushPkt is commonly used to signify end of a
					  message.
					* The endOfReadPkt is sent at the end of the packfile
					  transmission.
			*/

			log("cmd: stateless-connect")

			sshCmd, err := getSSHCommand(repo)
			if err != nil {
				return nil, false, err
			}
			if err := testSSH(sshCmd, host); err != nil {
				return nil, false, err
			}

			sshCmd = append(sshCmd, "-o", "SendEnv=GIT_PROTOCOL") // This allows us to request GIT_PROTOCOL v2

			sshExecCmd := fmt.Sprintf("%s '%s'", gitUploadPack, repository) // with stateless-connect, it's only fetches
			sshCmd = append(sshCmd, host, sshExecCmd)

			// Crafting ssh subprocess for fetches
			helper := exec.Command(sshCmd[0], sshCmd[1:]...) //nolint:gosec
			// Add env var for GIT_PROTOCOL v2
			helper.Env = append(os.Environ(), "GIT_PROTOCOL=version=2")
			helper.Stderr = os.Stderr

			// We want to inspect the helper's stdout for gittuf ref statuses
			helperStdOutPipe, err := helper.StdoutPipe()
			if err != nil {
				return nil, false, err
			}
			helperStdOut = &logReadCloser{readCloser: helperStdOutPipe, name: "ssh stdout"}

			// We want to interpose with the helper's stdin by passing in
			// extra refs etc.
			helperStdInPipe, err := helper.StdinPipe()
			if err != nil {
				return nil, false, err
			}
			helperStdIn = &logWriteCloser{writeCloser: helperStdInPipe, name: "ssh stdin"}

			if err := helper.Start(); err != nil {
				return nil, false, err
			}

			// Indicate connection established successfully
			if _, err := stdOutWriter.Write([]byte("\n")); err != nil {
				return nil, false, err
			}

			// Read from remote service
			// TODO: we may need nested infinite loops here
			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitPacket)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				// TODO: handle git protocol v0/v1
				// If server doesn't support v2, as soon as we connect,
				// it tells us the ref statuses

				if _, err := stdOutWriter.Write(output); err != nil {
					return nil, false, err
				}

				// check for end of message
				if bytes.Equal(output, flushPkt) {
					break
				}
			}

			// In protocol v2, this should now go to our parent process
			// requesting ls-refs
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

				// Check for end of message
				if bytes.Equal(input, flushPkt) {
					break
				}
			}

			helperStdOutScanner = bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitPacket)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				// In the curl transport, we also look out for endOfReadPkt
				// However, this has been a bit flakey
				// So when we see the flushPkt, we'll also write the
				// endOfReadPkt ourselves
				if !bytes.Equal(output, flushPkt) {
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

				if bytes.Equal(output, flushPkt) {
					// For a stateless connection, we must
					// also add the endOfRead packet
					// ourselves
					if _, err := stdOutWriter.Write(endOfReadPkt); err != nil {
						return nil, false, err
					}
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
				if len(input) == 0 {
					// We're done but we need to exit gracefully
					if err := helperStdIn.Close(); err != nil {
						return nil, false, err
					}
					if err := helperStdOut.Close(); err != nil {
						return nil, false, err
					}
					if err := helper.Wait(); err != nil {
						return nil, false, err
					}

					return gittufRefsTips, false, nil
				}

				if bytes.Equal(input, flushPkt) {
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
					wroteWants = true
				} else {
					if bytes.Contains(input, []byte("want")) {
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
					} else if bytes.Contains(input, []byte("have")) {
						idx := bytes.Index(input, []byte("have "))
						sha := string(bytes.TrimSpace(input[idx+len("have "):]))
						allHaves.Add(sha)
					}
				}

				if _, err := helperStdIn.Write(input); err != nil {
					return nil, false, err
				}

				// Read from remote if wants are done
				// We may need to scan multiple times for inputs, which is why
				// this flag is used
				if wroteWants {
					helperStdOutScanner := bufio.NewScanner(helperStdOut)
					helperStdOutScanner.Split(splitPacket)

					packReusedSeen := false // TODO: find something cleaner to terminate
					for helperStdOutScanner.Scan() {
						output := helperStdOutScanner.Bytes()

						// Send along to parent process
						if _, err := stdOutWriter.Write(output); err != nil {
							return nil, false, err
						}

						if len(output) > 4 {
							line := output[4:]

							if line[0] == 2 && bytes.Contains(line, []byte("pack-reused")) {
								packReusedSeen = true // we see this at the end
							}
						} else if bytes.Equal(output, flushPkt) {
							if packReusedSeen {
								if _, err := stdOutWriter.Write(endOfReadPkt); err != nil {
									return nil, false, err
								}
								break
							}

							// Go back for more input
							wroteWants = false
							break
						}
					}
				}
			}

		case bytes.HasPrefix(input, []byte("list for-push")):
			/*
				git: list for-push // wants to know remote ref statuses
				us: ssh...git-receive-pack
				ssh: list of refs
				us (to git): list of refs // trailing newline

				git: push cmds
				us: track list of push cmds, create RSL entry for each
				us (to ssh): push cmds (receive-pack format) // also track oldTip for eachRef
				us (to ssh): git pack-objects > ssh // include object range desired
			*/

			log("cmd: list for-push")

			sshCmd, err := getSSHCommand(repo)
			if err != nil {
				return nil, false, err
			}
			if err := testSSH(sshCmd, host); err != nil {
				return nil, false, err
			}

			sshCmd = append(sshCmd, "-o", "SendEnv=GIT_PROTOCOL") // This allows us to request GIT_PROTOCOL v2

			sshExecCmd := fmt.Sprintf("%s '%s'", gitReceivePack, repository) // with stateless-connect, it's only fetches
			sshCmd = append(sshCmd, host, sshExecCmd)

			// Crafting ssh subprocess for pushes
			helper := exec.Command(sshCmd[0], sshCmd[1:]...) //nolint:gosec
			// Add env var for GIT_PROTOCOL v2
			helper.Env = append(os.Environ(), "GIT_PROTOCOL=version=2")
			helper.Stderr = os.Stderr

			// We want to inspect the helper's stdout for gittuf ref statuses
			helperStdOutPipe, err := helper.StdoutPipe()
			if err != nil {
				return nil, false, err
			}
			helperStdOut = &logReadCloser{readCloser: helperStdOutPipe, name: "ssh stdout"}

			// We want to interpose with the helper's stdin by passing in
			// extra refs etc.
			helperStdInPipe, err := helper.StdinPipe()
			if err != nil {
				return nil, false, err
			}
			helperStdIn = &logWriteCloser{writeCloser: helperStdInPipe, name: "ssh stdin"}

			if err := helper.Start(); err != nil {
				return nil, false, err
			}

			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitPacket)

			// TODO: does this need a nested loop?
			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				// TODO: do we need endOfReadPkt check?
				if !bytes.Equal(output, flushPkt) {
					refAd := string(output[4:]) // remove length prefix
					refAd = strings.TrimSpace(refAd)

					refAdSplit := strings.Split(refAd, " ")
					ref := refAdSplit[1]
					if i := strings.IndexByte(ref, '\x00'); i > 0 {
						ref = ref[:i] // remove config string passed after null byte
					}
					tip := refAdSplit[0]

					if strings.HasPrefix(ref, gittufRefPrefix) {
						gittufRefsTips[ref] = tip
					}
					remoteRefTips[ref] = tip

					// We don't use ref, instead we use refAdSplit[1]. This
					// allows us to propagate remote capabilities to the parent
					// process
					if _, err := fmt.Fprintf(stdOutWriter, "%s %s\n", tip, refAdSplit[1]); err != nil {
						return nil, false, err
					}
				}

				if bytes.Equal(output, flushPkt) {
					// Add trailing new line as we're bridging git-receive-pack
					// output with git remote helper output
					if _, err := stdOutWriter.Write([]byte("\n")); err != nil {
						return nil, false, err
					}
					break
				}
			}

		case bytes.HasPrefix(input, []byte("push")):
			log("cmd: push")

			pushRefSpecs := []string{}
			for !bytes.Equal(input, []byte("\n")) {
				line := string(input)
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "push ")
				pushRefSpecs = append(pushRefSpecs, line)

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

			log("adding gittuf RSL entries")
			pushObjects := set.NewSet[string]()
			dstRefs := set.NewSet[string]()
			for i, refSpec := range pushRefSpecs {
				refSpecSplit := strings.Split(refSpec, ":")

				srcRef := refSpecSplit[0]
				srcRef = strings.TrimPrefix(srcRef, "+")

				dstRef := refSpecSplit[1]
				dstRefs.Add(dstRef)

				if dstRef == rsl.Ref {
					// We explicitly push the RSL ref below
					// because we need to know what its tip
					// will be after all other refs are
					// pushed.
					continue
				}

				if !strings.HasPrefix(dstRef, gittufRefPrefix) {
					// TODO: skipping propagation; invoke it once total instead of per ref
					if err := repo.RecordRSLEntryForReference(ctx, srcRef, true, rslopts.WithOverrideRefName(dstRef), rslopts.WithSkipCheckForDuplicateEntry(), rslopts.WithRecordLocalOnly()); err != nil {
						return nil, false, err
					}
				}

				oldTip := remoteRefTips[dstRef]
				if oldTip == "" {
					oldTip = gitinterface.ZeroHash.String()
				}

				newTipHash, err := repo.GetGitRepository().GetReference(srcRef)
				if err != nil {
					return nil, false, err
				}
				newTip := newTipHash.String()

				pushCmd := fmt.Sprintf("%s %s %s", oldTip, newTip, dstRef)
				if i == 0 {
					// report-status-v2 indicates we want the result for each pushed ref
					// atomic indicates either all must be successful or none
					// object-format indicates SHA-1 vs SHA-256 repo
					// agent indicates the version of the local git client (most of the time)
					// Note: we explicitly don't use the sideband here
					// because of inconsistencies between receive-pack
					// implementations in sending status messages.
					// TODO: check that server advertises all of these
					pushCmd = fmt.Sprintf("%s%s report-status-v2 atomic object-format=sha1 agent=git/%s", pushCmd, string('\x00'), gitVersion)
				}
				pushCmd += "\n"

				if _, err := helperStdIn.Write(packetEncode(pushCmd)); err != nil {
					return nil, false, err
				}

				if newTip != gitinterface.ZeroHash.String() {
					pushObjects.Add(newTip)
				}
				if oldTip != gitinterface.ZeroHash.String() {
					pushObjects.Add(fmt.Sprintf("^%s", oldTip)) // this is passed on to git rev-list to enumerate objects, and we're saying don't send the old objects
				}
			}

			// TODO: gittuf verify-ref for each dstRef; abort if
			// verification fails

			// TODO: find better way to evaluate if gittuf refs must
			// be pushed
			if len(gittufRefsTips) != 0 {
				oldTip, has := remoteRefTips[rsl.Ref]
				if !has {
					oldTip = gitinterface.ZeroHash.String()
				}

				newTipHash, err := repo.GetGitRepository().GetReference(rsl.Ref)
				if err != nil {
					return nil, false, err
				}
				newTip := newTipHash.String()
				log("RSL now has tip", newTip)

				pushCmd := fmt.Sprintf("%s %s %s\n", oldTip, newTip, rsl.Ref)
				if _, err := helperStdIn.Write(packetEncode(pushCmd)); err != nil {
					return nil, false, err
				}
				if newTip != gitinterface.ZeroHash.String() {
					pushObjects.Add(newTip)
				}
				if oldTip != gitinterface.ZeroHash.String() {
					pushObjects.Add(fmt.Sprintf("^%s", oldTip)) // this is passed on to git rev-list to enumerate objects, and we're saying don't send the old objects
				}
			}

			// Write the flush packet as we're done with ref processing
			if _, err := helperStdIn.Write(flushPkt); err != nil {
				return nil, false, err
			}

			cmd := exec.Command("git", "pack-objects", "--all-progress-implied", "--revs", "--stdout", "--thin", "--delta-base-offset", "--progress")

			// Write objects that must be pushed to stdin
			cmd.Stdin = bytes.NewBufferString(strings.Join(pushObjects.Contents(), "\n") + "\n") // the extra \n is used to indicate end of stdin entries

			// Redirect packfile bytes to remote service stdin
			cmd.Stdout = helperStdIn

			// Status updates get sent to parent process
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return nil, false, err
			}

			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitPacket)

			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				if len(output) == 4 {
					if _, err := stdOutWriter.Write([]byte("\n")); err != nil {
						return nil, false, err
					}

					if err := helperStdIn.Close(); err != nil {
						return nil, false, err
					}

					if err := helperStdOut.Close(); err != nil {
						return nil, false, err
					}

					return gittufRefsTips, true, nil
				}

				output = output[4:] // remove length prefix
				outputSplit := bytes.Split(output, []byte(" "))
				pushedRef := strings.TrimSpace(string(outputSplit[1]))
				if bytes.HasPrefix(output, []byte("ok")) {
					if dstRefs.Has(pushedRef) {
						if _, err := stdOutWriter.Write(output); err != nil {
							return nil, false, err
						}
					}
				} else if bytes.HasPrefix(output, []byte("ng")) {
					if dstRefs.Has(pushedRef) {
						output = bytes.TrimPrefix(output, []byte("ng"))
						output = append([]byte("error"), output...) // replace ng with error
						if _, err := stdOutWriter.Write(output); err != nil {
							return nil, false, err
						}
					}
				}
			}

			// Trailing newline for end of output
			if _, err := stdOutWriter.Write([]byte("\n")); err != nil {
				return nil, false, err
			}
		default:
			c := string(bytes.TrimSpace(input))
			if c == "" {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("unknown command %s to gittuf-ssh helper", c)
		}
	}

	// FIXME: we return in fetch and push when successful, need to assess when
	// this is reachable
	return nil, false, nil
}
