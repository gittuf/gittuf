#!/usr/bin/env bash
set -euxo pipefail
rm -rf client clone-test || true
################################################################################
# Step 1: Build Docker image for the Git server.
################################################################################

# Make sure there's a Dockerfile.gitserver in the current directory (adjust if needed).
docker build -t my-git-server -f Dockerfile.gitserver .

################################################################################
# Step 2: Run the Docker container (exposing port 9418).
################################################################################
docker run -d --name my-git-server -p 9418:9418 my-git-server

# Give the container a moment to start the git daemon
sleep 2

################################################################################
# Step 3: Build the git-remote-gittuf binary.
################################################################################
go build -o git-remote-gittuf ./internal/git-remote-gittuf
chmod +x git-remote-gittuf

# Make sure Git can find our remote helper by adding the current directory to PATH
export PATH="$(pwd):$PATH"

################################################################################
# Step 4: Integration Test — Push and Pull with the Docker-based Git server.
################################################################################

# 4a. Create a local repo and configure the remote to use the gittuf transport
mkdir client
cd client
git init

# This tells Git: "When pushing/pulling to 'origin', use git-remote-gittuf,
# but ultimately point to git://localhost:9418/test.git"
git remote add origin gittuf::git://localhost:9418/test.git

# 4b. Commit something and push
echo "Hello Gittuf (Docker Integration Test)" > file.txt
git add file.txt
git commit -m "Initial commit via gittuf transport"
git push --set-upstream origin master

echo "Push to Docker-based Git server successful!"

# 4c. Go back up a directory and clone from the server using gittuf transport
cd ..
git clone gittuf::git://localhost:9418/test.git clone-test

echo "Clone from Docker-based Git server successful! Listing clone-test commits:"
cd clone-test
git log --oneline

# (Optional) If your transport writes logs or RSL data, check them here.
# e.g., ls /path/to/rsl/logs

################################################################################
# Step 5: Clean up — Stop and remove the container.
################################################################################
cd ..
docker logs my-git-server || true  # Optional: see logs for debugging
docker stop my-git-server
docker rm my-git-server

echo "Docker-based Git server stopped and removed. Integration test completed successfully!"
