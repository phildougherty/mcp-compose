#!/bin/bash
# Build script for mcp-compose

set -e

# Get the directory of this script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR/.."

# Ensure all dependencies are downloaded
go mod download

# Build for the current platform
echo "Building mcp-compose..."
go build -o bin/mcp-compose ./cmd/mcp-compose

echo "Build complete. Binary available at bin/mcp-compose"
echo "To install locally: sudo cp bin/mcp-compose /usr/local/bin/"
