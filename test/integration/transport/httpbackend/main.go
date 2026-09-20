// SPDX-License-Identifier: Apache-2.0

// Command httpbackend serves one or more bare git repositories over HTTP
// using git-http-backend, for use in transport integration tests.
//
// Usage: httpbackend <project-root> <listen-addr>
package main

import (
	"log"
	"net/http"
	"net/http/cgi" // #nosec G504 -- httpoxy (CVE-2016-5386) affects Go <1.6.3; this workflow pins Go 1.27, and the binary only serves a local test repo, never exposed externally
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("usage: %s <project-root> <listen-addr>", os.Args[0]) //nolint:gosec // reviewer-approved: log injection risk accepted, see PR #1588
	}

	root := os.Args[1]
	addr := os.Args[2]

	out, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		log.Fatalf("unable to resolve git exec-path: %v", err)
	}
	backend := strings.TrimSpace(string(out)) + "/git-http-backend"

	handler := &cgi.Handler{
		Path: backend,
		Root: "/",
		Dir:  root,
		Env: []string{
			"GIT_PROJECT_ROOT=" + root,
			"GIT_HTTP_EXPORT_ALL=1",
		},
	}

	log.Printf("serving %s on %s", root, addr) //nolint:gosec // reviewer-approved: log injection risk accepted, see PR #1588
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
