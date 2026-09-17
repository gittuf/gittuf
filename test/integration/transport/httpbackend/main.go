// Command httpbackend serves one or more bare git repositories over HTTP
// using git-http-backend, for use in transport integration tests.
//
// Usage: httpbackend <project-root> <listen-addr>
package main

import (
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("usage: %s <project-root> <listen-addr>", os.Args[0])
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

	log.Printf("serving %s on %s", root, addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
