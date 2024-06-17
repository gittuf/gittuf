#!/usr/bin/env python

import os
import platform
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import difflib

REQUIRED_BINARIES = ["git", "ssh-keygen"]
SNIPPET_PATTERN = r"```bash\n([\s\S]*?)\n```"
EXPECTED_OUTPUT_FILENAME = "tester-expected-unix.txt"
GET_STARTED_FILENAME = "get-started.md"

# Validate that we have all the binaries required to run the test commands
def check_binaries():
    for p in REQUIRED_BINARIES:
        if not shutil.which(p):
            raise Exception(f"required command {p} not found")


def test_commands():
    curr_path = os.getcwd()
    docs_path = os.path.join(curr_path, "docs")
    testing_path = os.path.join(docs_path, "testing")
    keys_path = os.path.join(testing_path, "keys")
    os.chdir(testing_path)

    # Prepare testing directory
    match platform.system():
        case "Linux" | "Darwin":
            expected_output_file = os.path.realpath(os.path.join(testing_path, EXPECTED_OUTPUT_FILENAME))
        case "Windows":
            raise SystemExit("Windows is not supported at this time.")
        case _:
            raise SystemExit("Unknown platform.")
    
    get_started_file = os.path.realpath(os.path.join(docs_path, GET_STARTED_FILENAME))
    tmp_dir = os.path.realpath(tempfile.mkdtemp())
    os.chdir(tmp_dir)

    # Copy keys used to make git hashes deterministic    
    repo_path = os.path.join(tmp_dir, "repo")
    repo_keys_path = os.path.join(tmp_dir, "keys")
    shutil.copytree(keys_path, repo_keys_path)
    os.chdir(repo_keys_path)
    os.chmod("root", 0o0600)
    os.chmod("policy", 0o0600)
    os.chmod("developer", 0o0600)
    try:
        with open(expected_output_file) as fp1, open(get_started_file) as fp2:
            expected_output = fp1.read()
            get_started = fp2.read()
            snippets = re.findall(SNIPPET_PATTERN, get_started)

            for i, snippet in enumerate(snippets):
                snippets[i] = snippet.replace("$ ", "")
            script = "\nset -x\n{}".format("\n".join(snippets))
            script += "\ngittuf verify-ref main" # Workaround for non-deterministic hashes

            # Set some environment variables to make git hashes deterministic
            cmd_env = os.environ.copy()

            cmd_env["GIT_AUTHOR_NAME"] = "Jane Doe"
            cmd_env["GIT_AUTHOR_EMAIL"] = "jane.doe@example.com"
            cmd_env["GIT_AUTHOR_DATE"] = "2024-06-03T14:00:00.000Z"
            cmd_env["GIT_COMMITTER_NAME"] = "Jane Doe"
            cmd_env["GIT_COMMITTER_EMAIL"] = "jane.doe@example.com"
            cmd_env["GIT_COMMITTER_DATE"] = "2024-06-03T14:00:00.000Z"

            proc = subprocess.Popen(
                ["/bin/bash", "-c", script],
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                universal_newlines=True, env=cmd_env)
            stdout, _ = proc.communicate()

            if stdout != expected_output:
                difflist = list(difflib.Differ().compare(
                    expected_output.splitlines(),
                    stdout.splitlines()))
                raise SystemExit("Testing failed due to unexpected output:\n {}".format("\n".join(difflist)))
            else:
                print("Testing completed successfully.")
                        
    finally:
        os.chdir(curr_path)
        shutil.rmtree(tmp_dir)

if __name__ == "__main__":
    check_binaries()
    test_commands()
