#!/bin/bash
set -e

echo "Building KubeForge binaries using Docker..."

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed or not in PATH"
    exit 1
fi

# Build the builder image
docker build -t kubeforge-builder:latest -f Dockerfile.builder .

# Create a temporary container and extract the binaries
CONTAINER_ID=$(docker create kubeforge-builder:latest)
rm -rf dist/
mkdir -p dist/
docker cp $CONTAINER_ID:/dist/. dist/
docker rm $CONTAINER_ID

echo "Build completed successfully!"
echo ""
echo "The following files are available in the 'dist' directory:"
ls -la dist/
echo ""
echo "To install KubeForge, run: sudo ./dist/install.sh"

# To make this script executable, run:
# chmod +x docker-build.sh
# To run the script, use:
# ./docker-build.sh
# To build the Docker image, use:
# docker build -t kubeforge-builder:latest -f Dockerfile.builder .
# To run the Docker container, use:
# docker run --rm -v $(pwd):/dist kubeforge-builder:latest
# To copy files from the Docker container to the host, use:
# docker cp <container_id>:/dist/. ./dist/
# To remove the Docker container, use:
# docker rm <container_id>