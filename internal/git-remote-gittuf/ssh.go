// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/rsl"
)

var receivingPackfile = false

func handleSSH(_, url string) (map[string]string, bool, error) {
	url = strings.TrimPrefix(url, "ssh://")
	url = strings.TrimPrefix(url, "git+ssh://")
	url = strings.TrimPrefix(url, "ssh+git://")

	urlSplit := strings.Split(url, ":") // 0 is the connection [user@]host, 1 is the repo
	host := urlSplit[0]
	repository := urlSplit[1]

	stdInScanner := &logScanner{name: "git-remote-gittuf stdin", scanner: bufio.NewScanner(os.Stdin)}
	stdInScanner.Split(splitInput)

	var (
		helper         *exec.Cmd
		helperStdOut   io.ReadCloser
		helperStdIn    io.WriteCloser
		gittufRefsTips = map[string]string{}
		service        string
		isPush         bool
		remoteRefTips  = map[string]string{}
	)

	currentState := start // top level "menu" for the helper
	for stdInScanner.Scan() {
		command := stdInScanner.Bytes()

		log("packet mode:", isPacketMode)
		log("input post scanning:", command, string(command))

		switch currentState {
		case start:
			log("state: start")
			switch {
			case bytes.Equal(command, []byte("capabilities\n")):
				/*
					For SSH, we have several options wrt capabilities. First, we
					could just implement fetch and push. These are v0/v1
					protocols. The issue here is that while push is fine, fetch
					effectively fetches _all_ refs it sees on the remote via
					list. Additionally, using v2 protocol where possible seems
					good for efficiency improvements hinted at by the docs.

					The connect capability sets up a bidirectional connection
					with the server. It can handle both fetches and pushes;
					depending on what's happening, either upload-pack or
					receive-pack must be invoked on the server. This is fine for
					fetch operations. However, for push, we can tell the server
					to set refs/gittuf/<whatever> to the object. However, we do
					not control the invocation of git pack-objects --stdout. Git
					(which invokes us) invokes pack-objects separately, and
					routes its stdout into the transport's stdin to transmit the
					packfile bytes.

					In summary, we cannot use a combination of fetch and push,
					and we cannot use connect. What about stateless-connect?
					This is part of the v2 protocol and can only handle fetches
					at the moment. It's marked as experimental, which is
					something we want to be wary about with new Git versions.
					There may well be breaking changes here, given that the only
					intended user of this command is other Git tooling.

					stateless-connect is quite easy to work with to handle the
					fetch aspects. In addition, we implement the push
					capability. Here, Git tells us the refspecs that must be
					pushed. We are separately responsible for actually sending
					the packfile(s). So, the solution is that we create RSL
					entries for each requested ref, and include the gittuf
					objects in the packfile. Thus, we specify stateless-connect
					and push as the two capabilities supported by this helper.
				*/

				log("cmd: capabilities")
				if _, err := os.Stdout.Write([]byte("stateless-connect\npush\n\n")); err != nil {
					return nil, false, err
				}

			case bytes.HasPrefix(command, []byte("stateless-connect")):
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
				command = bytes.TrimSpace(command)
				commandSplit := bytes.Split(command, []byte(" "))
				// We can likely just set this to upload-pack
				service = string(commandSplit[1])
				log("found service", service)
				isPacketMode = true

				sshCmd, err := getSSHCommand()
				if err != nil {
					return nil, false, err
				}
				sshExecCmd := fmt.Sprintf("%s '%s'", service, repository)

				binary := sshCmd[0]
				args := []string{}
				if len(sshCmd) > 1 {
					// not just binary in the git config / env
					args = append(args, sshCmd[1:]...)
				}
				args = append(args, "-o", "SendEnv=GIT_PROTOCOL", host, sshExecCmd)

				helper = exec.Command(binary, args...)
				// We want to request GIT_PROTOCOL v2
				// https://git-scm.com/docs/protocol-v2
				helper.Env = append(os.Environ(), "GIT_PROTOCOL=version=2")
				helper.Stderr = os.Stderr

				// We want to inspect the helper's stdout for the gittuf ref
				// statuses
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
				if _, err := os.Stdout.Write([]byte("\n")); err != nil {
					return nil, false, err
				}

				flushPktSeen := false
				for {
					if flushPktSeen {
						break
					}
					// TODO: why do we need this nested infinite loop?

					helperStdOutScanner := bufio.NewScanner(helperStdOut)
					helperStdOutScanner.Split(splitOutput)

					for helperStdOutScanner.Scan() {
						output := helperStdOutScanner.Bytes()

						if bytes.Contains(output, []byte(gittufRefPrefix)) {
							// Sometimes we may have GIT_PROTOCOL v0/v1
							// response, where refs are advertised right away
							refAd := strings.TrimSpace(string(output)[4:]) // remove the length prefix
							if i := strings.IndexByte(refAd, '\x00'); i > 0 {
								// this checks if the gittuf entry is the very first
								// returned (unlikely because of HEAD)
								refAd = refAd[:i] // drop everything from null byte onwards
							}
							refAdSplit := strings.Split(refAd, " ")

							if strings.HasPrefix(refAdSplit[1], gittufRefPrefix) {
								gittufRefsTips[refAdSplit[1]] = refAdSplit[0]
							}
						}

						if _, err := os.Stdout.Write(output); err != nil {
							return nil, false, err
						}

						if bytes.Equal(output, flushPkt) {
							flushPktSeen = true
							break
						}
					}
				}

				currentState = serviceRouter

			case bytes.HasPrefix(command, []byte("list for-push")):
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
				isPush = true

				sshCmd, err := getSSHCommand()
				if err != nil {
					return nil, false, err
				}

				service = gitReceivePack
				sshExecCmd := fmt.Sprintf("%s '%s'", service, repository)

				binary := sshCmd[0]
				args := []string{}
				if len(sshCmd) > 1 {
					// not just binary
					args = append(args, sshCmd[1:]...)
				}
				args = append(args, "-o", "SendEnv=GIT_PROTOCOL", host, sshExecCmd)

				helper = exec.Command(binary, args...)
				helper.Env = append(os.Environ(), "GIT_PROTOCOL=version=2")
				helper.Stderr = os.Stderr

				// We want to inspect the helper's stdout for the gittuf ref
				// statuses.
				helperStdOutPipe, err := helper.StdoutPipe()
				if err != nil {
					return nil, false, err
				}
				helperStdOut = &logReadCloser{name: "ssh stdout", readCloser: helperStdOutPipe}

				// We want to interpose with the helper's stdin to transmit the
				// packfile
				helperStdInPipe, err := helper.StdinPipe()
				if err != nil {
					return nil, false, err
				}
				helperStdIn = &logWriteCloser{name: "ssh stdin", writeCloser: helperStdInPipe}

				if err := helper.Start(); err != nil {
					return nil, false, err
				}

				log("list for-push returned:")
				flushPktSeen := false
				for {
					if flushPktSeen {
						break
					}

					helperStdOutScanner := bufio.NewScanner(helperStdOut)
					helperStdOutScanner.Split(splitOutput)

					for helperStdOutScanner.Scan() {
						output := helperStdOutScanner.Bytes()

						// handle gittuf refs
						if !bytes.Equal(output, flushPkt) && !bytes.Equal(output, endOfReadPkt) {
							refAd := bytes.TrimSpace(output[4:]) // remove the length prefix
							refAdSplit := bytes.Split(refAd, []byte{' '})

							ref := ""
							if i := bytes.IndexByte(refAdSplit[1], '\000'); i >= 0 {
								ref = string(refAdSplit[1][:i])
							} else {
								ref = string(refAdSplit[1])
							}

							tip := string(refAdSplit[0])

							if strings.HasPrefix(ref, gittufRefPrefix) {
								gittufRefsTips[ref] = tip
							}

							remoteRefTips[ref] = tip

							// We don't use `ref` here so that we can propagate
							// the server's capabilities to the client
							if _, err := os.Stdout.Write([]byte(fmt.Sprintf("%s %s\n", tip, refAdSplit[1]))); err != nil {
								return nil, false, err
							}
						}

						if bytes.Equal(output, flushPkt) {
							flushPktSeen = true
							// Add the trailing new line; recall that we're
							// bridging the remote service with what Git expects
							// of the transport
							if _, err := os.Stdout.Write([]byte("\n")); err != nil {
								return nil, false, err
							}
							break
						}
					}
				}

			case bytes.HasPrefix(command, []byte("push")):
				log("cmd: push")
				pushRefSpecs := []string{}

				for {
					if bytes.Equal(command, []byte("\n")) {
						break
					}

					command = bytes.TrimSpace(command)
					pushRefSpecs = append(pushRefSpecs, string(bytes.TrimPrefix(command, []byte("push "))))

					if !stdInScanner.Scan() {
						break
					}
					command = stdInScanner.Bytes()
				}

				pushObjects := set.NewSet[string]()
				log("adding gittuf RSL entries")
				for i, refSpec := range pushRefSpecs {
					refSpecSplit := strings.Split(refSpec, ":")
					srcRef := refSpecSplit[0]
					dstRef := refSpecSplit[1]
					// TODO: check RSL is updated against remote
					// The best way to fetch the RSL first may be to just
					// `git fetch <remoteName> refs/gittuf/*:refs/gittuf/*`
					// This will result in a _separate_ instance of the
					// transport focused on pull (side note: do servers block
					// concurrent connections? I don't see why they would) and
					// we must address the issue of both the transport and git
					// itself trying to update-ref when the fetched ref is for
					// gittuf. See main.go for more details.

					// We don't want to write to os.Stdout, that'll confuse Git
					cmd := exec.Command("gittuf", "rsl", "record", "--dst-ref", dstRef, srcRef)
					cmd.Stderr = os.Stderr
					cmd.Stdout = os.Stderr
					cmd.Stdin = os.Stdin
					if err := cmd.Run(); err != nil {
						return nil, false, err
					}

					cmd = exec.Command("git", "rev-parse", srcRef)
					output, err := cmd.Output() // Output() redirects streams for us
					if err != nil {
						return nil, false, err
					}

					oldTip := remoteRefTips[dstRef]
					if oldTip == "" {
						oldTip = zeroHash
					}
					newTip := string(bytes.TrimSpace(output))

					pushCmd := fmt.Sprintf("%s %s %s", oldTip, newTip, dstRef)
					if i == 0 {
						// report-status-v2 indicates we want the result for each pushed ref
						// atomic indicates either all must be successful or none
						// object-format indicates SHA-1 vs SHA-256 repo
						// agent indicates the version of the local git client (most of the time)
						// Note: we explicitly don't use the sideband here
						// because of incosistencies between receive-pack
						// implementations in sending status messages.
						// TODO: check that server advertises all of these
						pushCmd = fmt.Sprintf("%s%s report-status-v2 atomic object-format=sha1 agent=git/%s", pushCmd, string('\x00'), gitVersion)
					}
					pushCmd += "\n"

					if _, err := helperStdIn.Write(packetEncode(pushCmd)); err != nil {
						return nil, false, err
					}

					if newTip != zeroHash {
						pushObjects.Add(newTip)
					}
					if oldTip != zeroHash {
						pushObjects.Add(fmt.Sprintf("^%s", oldTip)) // this is passed on to git rev-list to enumerate objects, and we're saying don't send the old objects
					}
				}

				// TODO: gittuf verify-ref for each dstRef; abort if
				// verification fails

				cmd := exec.Command("git", "rev-parse", rsl.Ref) //nolint:gosec
				output, err := cmd.Output()
				if err != nil {
					return nil, false, err
				}

				oldTip := remoteRefTips[rsl.Ref]
				newTip := string(bytes.TrimSpace(output))

				pushCmd := fmt.Sprintf("%s %s %s\n", oldTip, newTip, rsl.Ref)
				if _, err := helperStdIn.Write(packetEncode(pushCmd)); err != nil {
					return nil, false, err
				}
				if _, err := helperStdIn.Write(flushPkt); err != nil {
					return nil, false, err
				}
				if newTip != zeroHash {
					pushObjects.Add(newTip)
				}
				if oldTip != zeroHash {
					pushObjects.Add(fmt.Sprintf("^%s", oldTip)) // this is passed on to git rev-list to enumerate objects, and we're saying don't send the old objects
				}

				cmd = exec.Command("git", "pack-objects", "--all-progress-implied", "--revs", "--stdout", "--thin", "--delta-base-offset", "--progress")
				cmd.Stdin = bytes.NewBufferString(strings.Join(pushObjects.Contents(), "\n") + "\n") // the extra \n is used to indicate end of stdin entries
				cmd.Stdout = helperStdIn                                                             // redirect packfile bytes to the remote service
				cmd.Stderr = os.Stderr                                                               // status updates get sent to Git

				if err := cmd.Run(); err != nil {
					return nil, false, err
				}

				isPacketMode = true
				helperStdOutScanner := bufio.NewScanner(helperStdOut)
				helperStdOutScanner.Split(splitOutput)

				for helperStdOutScanner.Scan() {
					output := helperStdOutScanner.Bytes()
					output = output[4:] // remove the packet length prefix
					if bytes.HasPrefix(output, []byte("ok")) {
						if !bytes.Contains(output, []byte(gittufRefPrefix)) {
							if _, err := os.Stdout.Write(output); err != nil {
								return nil, false, err
							}
						}
					} else if bytes.HasPrefix(output, []byte("ng")) {
						output = bytes.TrimPrefix(output, []byte("ng"))
						output = append([]byte("error"), output...) // replace ng with error for our invoker

						if bytes.Contains(output, []byte(gittufRefPrefix)) {
							// error is in updating gittuf, send it to stderr
							if _, err := os.Stderr.Write(output); err != nil {
								return nil, false, err
							}
						} else {
							output = append(output, '\n')
							if _, err := os.Stdout.Write(output); err != nil {
								return nil, false, err
							}
						}
					}

					if len(output) == 0 {
						// This tells us that the 4 bytes we removed earlier
						// was a "special" packaet such as flush
						if _, err := os.Stdout.Write([]byte("\n")); err != nil {
							return nil, false, err
						}
						currentState = packfileDone
						break
					}
				}

			default:
				c := string(bytes.TrimSpace(command))
				if c == "" {
					return nil, isPush, nil
				}
				return nil, false, fmt.Errorf("unknown command %s to gittuf-ssh helper", c)
			}

		case serviceRouter:
			log("state: service-router")
			switch service { //nolint:gocritic
			case gitUploadPack:
				if bytes.Contains(command, []byte("command=ls-refs")) {
					currentState = lsRefs
				} else if bytes.Contains(command, []byte("command=fetch")) || bytes.Contains(command, []byte("want")) {
					currentState = requestingWants
					// we see "want" when we're not in protocol v2
					// also, here, the entire list of wants is nested in packet format
					// pktlength(pktlength(line)...)
					// we have to recognize we're in v0/1 and store, interpose, recalculate packet encoding, then send to ssh subprocess
				}
			}

			if _, err := helperStdIn.Write(command); err != nil {
				return nil, false, err
			}

			// Right now, we don't need to wait for a response here, we check
			// what command of the git service we're invoking and go to that
			// state, this is a "routing" state. THIS MAY CHANGE!

		case lsRefs:
			// https://git-scm.com/docs/protocol-v2#_ls_refs
			log("cmd: ls-refs")
			if bytes.Equal(command, flushPkt) {
				// add the gittuf ref-prefix right before the flushPkt
				log("adding ref-prefix for refs/gittuf/")
				gittufRefPrefixCommand := fmt.Sprintf("ref-prefix %s\n", gittufRefPrefix)
				if _, err := helperStdIn.Write(packetEncode(gittufRefPrefixCommand)); err != nil {
					return nil, false, err
				}

				currentState = lsRefsResponse
			}

			if _, err := helperStdIn.Write(command); err != nil {
				return nil, false, err
			}

			// After writing flush to stdin, we can get the advertised refs
			if currentState == lsRefsResponse {
				helperStdOutScanner := bufio.NewScanner(helperStdOut)
				helperStdOutScanner.Split(splitOutput)

				for helperStdOutScanner.Scan() {
					output := helperStdOutScanner.Bytes()

					// handle gittuf refs
					if !bytes.Equal(output, flushPkt) && !bytes.Equal(output, endOfReadPkt) {
						refAd := strings.TrimSpace(string(output)[4:]) // remove the length prefix
						if i := strings.IndexByte(refAd, '\x00'); i > 0 {
							// this checks if the gittuf entry is the very first
							// returned (unlikely because of HEAD)
							refAd = refAd[:i] // drop everything from null byte onwards
						}
						refAdSplit := strings.Split(refAd, " ")

						if strings.HasPrefix(refAdSplit[1], gittufRefPrefix) {
							gittufRefsTips[refAdSplit[1]] = refAdSplit[0]
						}
					}

					if _, err := os.Stdout.Write(output); err != nil {
						return nil, false, err
					}

					if bytes.Equal(output, flushPkt) {
						if _, err := os.Stdout.Write(endOfReadPkt); err != nil {
							return nil, false, err
						}
						break
					}
				}

				currentState = serviceRouter // go back to service's "router"
			}

		case requestingWants:
			// https://git-scm.com/docs/protocol-v2#_fetch
			log("cmd: fetch")

			gittufWantsDone := false
			for {
				// Enter client-side multi-line batch for negotiation
				if bytes.Contains(command, []byte("want")) {
					if !gittufWantsDone {
						gittufWants, err := getGittufWants(gittufRefsTips)
						if err == nil {
							for _, want := range gittufWants {
								wantCmd := fmt.Sprintf("want %s\n", want)
								log("gittuf want cmd", wantCmd)
								if _, err := helperStdIn.Write(packetEncode(wantCmd)); err != nil {
									return nil, false, err
								}
							}
						} else {
							for _, tip := range gittufRefsTips {
								wantCmd := fmt.Sprintf("want %s\n", tip)
								log("gittuf want cmd", wantCmd)
								if _, err := helperStdIn.Write(packetEncode(wantCmd)); err != nil {
									return nil, false, err
								}
							}
						}
					}
					gittufWantsDone = true
				} else if bytes.Equal(command, flushPkt) {
					if _, err := helperStdIn.Write(command); err != nil {
						return nil, false, err
					}
					break
				}

				if _, err := helperStdIn.Write(command); err != nil {
					return nil, false, err
				}

				stdInScanner.Scan()
				command = stdInScanner.Bytes()
			}

			currentState = requestingWantsResponse

			helperStdOutScanner := bufio.NewScanner(helperStdOut)
			helperStdOutScanner.Split(splitOutput)

			packReusedSeen := false
			for helperStdOutScanner.Scan() {
				output := helperStdOutScanner.Bytes()

				if _, err := os.Stdout.Write(output); err != nil {
					return nil, false, err
				}

				log("writing out:", output, string(output))

				if len(output) > 4 {
					// there's an actual packet line

					line := output[4:]

					if !receivingPackfile && bytes.HasPrefix(line, []byte("packfile")) {
						receivingPackfile = true
					} else if line[0] == 2 {
						// sideband channel 2: progress message
						if bytes.Contains(line, []byte("pack-reused")) {
							// we see this at the end
							packReusedSeen = true
						}
					}
				} else if bytes.Equal(output, flushPkt) {
					if receivingPackfile && packReusedSeen {
						if _, err := os.Stdout.Write(endOfReadPkt); err != nil {
							return nil, false, err
						}
						break
					}
					// We have more wants
					currentState = requestingWants
					break
				}
			}

			if packReusedSeen && currentState == requestingWantsResponse {
				// We don't want to accidentally override a requestingWants
				// here
				currentState = packfileDone
			}
		}

		if currentState == packfileDone {
			break
		}
	}

	if !isPush {
		// FIXME: closing these pipes caused trouble during a push, likely
		// because we weren't done transmitting objects?
		// at the same time, it doesn't hang on these pipes closing either so...
		if err := helperStdIn.Close(); err != nil {
			return nil, false, err
		}

		if err := helperStdOut.Close(); err != nil {
			return nil, false, err
		}

		if err := helper.Wait(); err != nil {
			return nil, false, err
		}
	}

	return gittufRefsTips, isPush, nil
}
