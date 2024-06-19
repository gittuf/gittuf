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

REQUIRED_BINARIES = ["git", "gittuf", "ssh-keygen"]
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
    get_started_file = os.path.realpath(os.path.join(docs_path, GET_STARTED_FILENAME))
    os.chdir(testing_path)

    # Check for supported platform
    match platform.system():
        case "Linux" | "Darwin":
            expected_output_file = os.path.realpath(os.path.join(testing_path, EXPECTED_OUTPUT_FILENAME))
        case "Windows":
            raise SystemExit("Windows is not supported at this time.")
        case _:
            raise SystemExit("Unknown platform.")
    
    # Prepare temporary directory
    tmp_dir = os.path.realpath(tempfile.mkdtemp())
    os.chdir(tmp_dir)

    try:
        with open(expected_output_file) as fp1, open(get_started_file) as fp2:
            # Read in the get_started.md and expected output files
            expected_output = fp1.read()
            get_started = fp2.read()
            snippets = re.findall(SNIPPET_PATTERN, get_started)

            # Prepend the set command to echo commands and exit in case of
            # failure
            script = "\nset -xe\n{}".format("\n".join(snippets))
            script += "\ngittuf verify-ref main" # Workaround for non-deterministic hashes

            # Set some environment variables to control commit creation
            cmd_env = os.environ.copy()

            cmd_env["GIT_AUTHOR_NAME"] = "Jane Doe"
            cmd_env["GIT_AUTHOR_EMAIL"] = "jane.doe@example.com"
            cmd_env["GIT_AUTHOR_DATE"] = "2024-06-03T14:00:00.000Z"
            cmd_env["GIT_COMMITTER_NAME"] = "Jane Doe"
            cmd_env["GIT_COMMITTER_EMAIL"] = "jane.doe@example.com"
            cmd_env["GIT_COMMITTER_DATE"] = "2024-06-03T14:00:00.000Z"

            # Execute generated script
            proc = subprocess.Popen(
                ["/bin/bash", "-c", script],
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                universal_newlines=True, env=cmd_env)
            stdout, _ = proc.communicate()

            # Compare and notify user of result
            if stdout != expected_output:
                difflist = list(difflib.Differ().compare(
                    expected_output.splitlines(),
                    stdout.splitlines()))
                raise SystemExit("Testing failed due to unexpected output:\n {}".format("\n".join(difflist)))
            else:
                print("Testing completed successfully.")
                        
    finally:
        # Cleanup
        os.chdir(curr_path)
        shutil.rmtree(tmp_dir)

if __name__ == "__main__":
    check_binaries()
    test_commands()
