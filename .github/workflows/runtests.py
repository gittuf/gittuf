#!/usr/bin/env python3
import os
import subprocess
import shlex

def run_command(cmd, expected_retcode=0):
    """Runs the supplied command and checks for the expected return code."""
    retcode = subprocess.call(shlex.split(cmd))
    if retcode != expected_retcode:
        raise Exception(f"Expected {expected_retcode} from process but it exited with {retcode}.")

# Constants
GITTUF_TRANSPORT_BINARY = "git-remote-gittuf"
TEST_REPO_URL_HTTPS = "gittuf::https://github.com/zsun6/gittuf-remote-test_repo"
TEST_CLONE_DIR = "test_clone"
TEST_REMOTE_NAME = "origin"
GITTUF_TEST_BRANCH = "test-branch"

# Verify if the gittuf binary is installed
# try:
#     run_command(f"command -v {GITTUF_TRANSPORT_BINARY}")
#     print("Git-Remote-Gittuf binary is installed.")
# except Exception:
#     print(f"Error: {GITTUF_TRANSPORT_BINARY} is not installed or not in PATH.")
#     exit(1)

# Clean up from previous runs
if os.path.isdir(TEST_CLONE_DIR):
    subprocess.call(["rm", "-rf", TEST_CLONE_DIR])
    print("Cleaned up previous test environment.")

REPO_URL = TEST_REPO_URL_HTTPS

# Test cloning the repository
print(f"Testing clone using {REPO_URL}...")
run_command(f"git clone {REPO_URL} {TEST_CLONE_DIR}")
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
run_command(f"git fetch {TEST_REMOTE_NAME}")

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
