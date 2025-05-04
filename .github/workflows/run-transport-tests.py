#!/usr/bin/env python3
"""
A script to:
    1. Initialize a bare Git repository under ./repos/repo.git.
    2. Enable HTTP receive-pack (push) for that repo.
    3. Create an initial commit with "hello test1" in README.md and push to the bare repo.
    4. Launch an HTTP Git server (using git-http-backend via CGI) on port 8000.
    5. Clone the repo over HTTP, update README.md to "hello world", commit, and push back.
    6. Verify the push by cloning via file:// and checking the README.md contents.

purpose of script:
    To show that building HTTP git server (that supports push/pull) is possible but adding "gittuff::" creates an infinite stall for uknown reason.
"""
import os
import shutil
import threading
import subprocess
import sys
import time

# Settings
env = os.environ
PORT = 8000
BASE_DIR = os.getcwd()
BARE_PARENT = os.path.join(BASE_DIR, 'repos')
BARE_REPO = os.path.join(BARE_PARENT, 'repo.git')
WORK_INITIAL = os.path.join(BASE_DIR, 'work1')
WORK_HTTP = os.path.join(BASE_DIR, 'work2')
WORK_VERIFY = os.path.join(BASE_DIR, 'verify')
CGI_DIR = os.path.join(BASE_DIR, 'cgi-bin')

# set a fallback identity so commits don’t fail in CI
subprocess.check_call(["git", "config", "--global", "user.email", "action@github.com"])
subprocess.check_call(["git", "config", "--global", "user.name",  "GitHub Action"])

def run(cmd, cwd=None):
    print(f"[*] Running: {' '.join(cmd)}")
    subprocess.check_call(cmd, cwd=cwd)


def run_http_server(repo_root, port):
    # Prepare CGI directory and link git-http-backend
    os.makedirs(CGI_DIR, exist_ok=True)
    backend = shutil.which('git-http-backend')
    if not backend:
        print('[!] git-http-backend not found in PATH')
        sys.exit(1)
    link = os.path.join(CGI_DIR, 'git-http-backend')
    if os.path.exists(link):
        os.remove(link)
    os.symlink(backend, link)

    # Set environment for smart HTTP
    env['GIT_PROJECT_ROOT'] = repo_root
    env['GIT_HTTP_EXPORT_ALL'] = '1'
    # Enable receive-pack via config and/or env
    env['GIT_HTTP_RECEIVEPACK'] = '1'

    from http.server import HTTPServer, CGIHTTPRequestHandler
    os.chdir(BASE_DIR)
    httpd = HTTPServer(('127.0.0.1', port), CGIHTTPRequestHandler)
    print(f"[*] Serving HTTP Git on http://127.0.0.1:{port}/cgi-bin/git-http-backend/repo.git")
    httpd.serve_forever()


def main():
    # Cleanup previous runs
    for path in [BARE_PARENT, WORK_INITIAL, WORK_HTTP, WORK_VERIFY, CGI_DIR]:
        if os.path.exists(path):
            shutil.rmtree(path)

    # 1. Init bare repo
    os.makedirs(BARE_PARENT, exist_ok=True)
    run(['git', 'init', '--bare', 'repo.git'], cwd=BARE_PARENT)

    # 2. Enable HTTP receive-pack in repo config
    run(['git', 'config', 'http.receivepack', 'true'], cwd=BARE_REPO)

    # 3. Initial commit
    run(['git', 'clone', BARE_REPO, WORK_INITIAL])
    with open(os.path.join(WORK_INITIAL, 'README.md'), 'w') as f:
        f.write('hello test1\n')
    run(['git', 'add', 'README.md'], cwd=WORK_INITIAL)
    run(['git', 'commit', '-m', 'Initial commit: hello test1'], cwd=WORK_INITIAL)
    run(['git', 'push', 'origin', 'master'], cwd=WORK_INITIAL)

    # 4. Start HTTP Git server in background
    server_thread = threading.Thread(
        target=run_http_server, args=(BARE_PARENT, PORT), daemon=True
    )
    server_thread.start()
    time.sleep(1)

    # 5. Clone over HTTP, update, and push
    http_url = f'http://127.0.0.1:{PORT}/cgi-bin/git-http-backend/repo.git'
    run(['git', 'clone', http_url, WORK_HTTP])
    with open(os.path.join(WORK_HTTP, 'README.md'), 'w') as f:
        f.write('hello world\n')
    run(['git', 'add', 'README.md'], cwd=WORK_HTTP)
    run(['git', 'commit', '-m', 'Update README to hello world'], cwd=WORK_HTTP)
    run(['git', 'push', 'origin', 'master'], cwd=WORK_HTTP)

    # 6. Verification
    run(['git', 'clone', BARE_REPO, WORK_VERIFY])
    with open(os.path.join(WORK_VERIFY, 'README.md'), 'r') as f:
        content = f.read().strip()
    if content == 'hello world':
        print(f"[+] Push successful! README.md contains: '{content}'")
    else:
        print(f"[-] Push failed. README.md contains: '{content}'")


if __name__ == '__main__':
    # Ensure git-http-backend exists
    if not shutil.which('git-http-backend'):
        print('[!] git-http-backend not found. Is Git installed?')
        sys.exit(1)
    main()
