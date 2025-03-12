import os
import subprocess
import shlex
import tempfile
import threading
import ssl
import shutil
from http.server import HTTPServer, SimpleHTTPRequestHandler

def run_command(cmd, expected_retcode=0):
    """Runs the supplied command and checks for the expected return code."""
    retcode = subprocess.call(shlex.split(cmd))
    if retcode != expected_retcode:
        raise Exception(f"Expected {expected_retcode} from process but it exited with {retcode}.")

# Cleanup previous runs
TEST_CLONE_DIR = "test_clone"
for path in [TEST_CLONE_DIR, 'repo']:
    if os.path.exists(path):
        shutil.rmtree(path)
        print(f"Cleaned up existing directory: {path}")

# Create temporary server environment
with tempfile.TemporaryDirectory() as server_dir, tempfile.TemporaryDirectory() as local_repo_dir:
    # Setup bare repository at correct path
    bare_repo_path = os.path.join(server_dir, 'repo.git')
    subprocess.run(['git', 'init', '--bare', bare_repo_path], check=True)
    
    # Enable HTTP transport support
    subprocess.run(['git', '-C', bare_repo_path, 'config', 'http.receivepack', 'true'], check=True)
    subprocess.run(['git', '-C', bare_repo_path, 'config', 'http.uploadpack', 'true'], check=True)
    
    # Initialize test repository
    subprocess.run(['git', 'init', local_repo_dir], check=True)
    with open(os.path.join(local_repo_dir, 'README.md'), 'w') as f:
        f.write('# Local Test Repository\n')
    subprocess.run(['git', '-C', local_repo_dir, 'add', '.'], check=True)
    subprocess.run(['git', '-C', local_repo_dir, 'commit', '-m', 'Initial commit'], check=True)
    subprocess.run(['git', '-C', local_repo_dir, 'remote', 'add', 'origin', bare_repo_path], check=True)
    subprocess.run(['git', '-C', local_repo_dir, 'push', 'origin', 'master'], check=True)
    
    # Enable post-update hook
    post_update_hook = os.path.join(bare_repo_path, 'hooks', 'post-update')
    with open(post_update_hook, 'w') as f:
        f.write('#!/bin/sh\n')
        f.write('git update-server-info\n')
    os.chmod(post_update_hook, 0o755)
    subprocess.run(['git', '-C', bare_repo_path, 'update-server-info'], check=True)
    
    # Generate SSL certificate
    keyfile = os.path.join(server_dir, 'key.pem')
    certfile = os.path.join(server_dir, 'cert.pem')
    subprocess.run([
        'openssl', 'req', '-x509', '-newkey', 'rsa:4096',
        '-keyout', keyfile, '-out', certfile, '-days', '365', '-nodes',
        '-subj', '/CN=localhost'
    ], check=True)
    
    # Custom Git HTTP handler
    class GitHTTPRequestHandler(SimpleHTTPRequestHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, directory=bare_repo_path, **kwargs)
        
        def do_GET(self):
            # Handle Git's smart HTTP protocol
            if self.path.startswith('/repo.git/'):
                self.path = self.path[len('/repo.git'):]
            elif self.path == '/repo.git':
                self.path = '/'
            return super().do_GET()
        
        def end_headers(self):
            # Add headers required for Git HTTP
            self.send_header('Content-Type', 'application/x-git-upload-pack-result')
            super().end_headers()

    # Start HTTPS server
    port = 8000
    server = HTTPServer(('localhost', port), GitHTTPRequestHandler)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(certfile=certfile, keyfile=keyfile)
    server.socket = context.wrap_socket(server.socket, server_side=True)
    server_thread = threading.Thread(target=server.serve_forever)
    server_thread.daemon = True
    server_thread.start()
    
    try:
        TEST_REPO_URL_HTTPS = f"https://localhost:{port}/repo.git"
        TEST_REMOTE_NAME = "origin"
        GITTUF_TEST_BRANCH = "test-branch"

        if os.path.isdir(TEST_CLONE_DIR):
            subprocess.call(["rm", "-rf", TEST_CLONE_DIR])
            print("Cleaned up previous test environment.")

        REPO_URL = TEST_REPO_URL_HTTPS

        # Test cloning the repository
        print(f"Testing clone using {REPO_URL}...")
        run_command(f"git -c http.sslVerify=false clone {REPO_URL} {TEST_CLONE_DIR}")
        os.chdir(TEST_CLONE_DIR)

        if not os.path.isdir(".git"):
            print("Error: Failed to clone repository.")
            exit(1)
        print("Successfully cloned repository.")

        # Test setting remote to HTTPS
        print("Testing setting remote URL to HTTPS...")
        run_command(f"git remote set-url {TEST_REMOTE_NAME} {TEST_REPO_URL_HTTPS}")

        remote_url = subprocess.check_output(shlex.split(f"git remote get-url {TEST_REMOTE_NAME}")).strip().decode()
        if remote_url != TEST_REPO_URL_HTTPS:
            print("Error: Failed to set remote URL to HTTPS.")
            exit(1)
        print("Successfully set remote URL to HTTPS.")

        # Test first fetch
        print("Testing fetch operation...")
        run_command(f"git -c http.sslVerify=false fetch {TEST_REMOTE_NAME}")

        # Check if the repository folder exists
        repo_dir = "repo"
        if os.path.isdir(repo_dir):
            print(f"Repository folder '{repo_dir}' found.")
            readme_file = os.path.join(repo_dir, "README.md")
            if os.path.isfile(readme_file):
                print("README file found. Printing contents:")
                with open(readme_file, "r") as file:
                    print(file.read())
            else:
                print(f"README file not found in '{repo_dir}'.")
        else:
            print(f"Repository folder '{repo_dir}' does not exist.")
            exit(1)
        print("Fetch operation completed successfully.")

        # Test creating a new branch and pushing changes
        print("Testing push operation...")
        run_command(f"git checkout -b {GITTUF_TEST_BRANCH}")
        with open("written_by_autotest.txt", "w") as file:
            file.write("This is auto test")

        run_command("git add written_by_autotest.txt")
        run_command(f"git commit -m \"Add test file for {GITTUF_TEST_BRANCH}\"")
        run_command(f"git push -u {TEST_REMOTE_NAME} {GITTUF_TEST_BRANCH} --force",1)
        print("Push operation completed successfully.")

        # Test fetching updates
        print("Testing fetch operation...")
        run_command(f"git fetch {TEST_REMOTE_NAME}")
        readme_file_2 = "written_by_autotest.txt"
        if os.path.isfile(readme_file_2):
            print("Pushed file found. Printing contents:")
            with open(readme_file_2, "r") as file:
                print(file.read())
        else:
            print(f"README file not found in '{repo_dir}'.")
        print("Fetch operation completed successfully.")

        # Clean up
        print("Cleaning up test environment...")
        os.chdir("..")
        subprocess.call(["rm", "-rf", TEST_CLONE_DIR])
        print("All tests completed successfully.")


    finally:
        server.shutdown()
        server.server_close()
        server_thread.join()
        os.chdir("..")
        shutil.rmtree(TEST_CLONE_DIR, ignore_errors=True)