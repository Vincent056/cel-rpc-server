#!/bin/bash

# Build the scanner container image
IMAGE_NAME="ghcr.io/vincent056/cel-scanner"
VERSION="latest"

echo "Building CEL scanner container..."

# Build the image
docker build -f scanner/Dockerfile -t ${IMAGE_NAME}:${VERSION} .

echo "Build complete: ${IMAGE_NAME}:${VERSION}"
echo ""
echo "To push the image:"
echo "  docker push ${IMAGE_NAME}:${VERSION}"
echo ""
echo "To use a local registry:"
echo "  docker tag ${IMAGE_NAME}:${VERSION} localhost:5000/cel-scanner:latest"
echo "  docker push localhost:5000/cel-scanner:latest" 