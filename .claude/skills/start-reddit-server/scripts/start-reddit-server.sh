#!/bin/bash
# run-server.sh - Run reddit-server with file logging and API key from .env

set -euo pipefail

# Source environment variables (.env should contain TEST_KEY, REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET)
if [ -f .env ]; then
    source .env
else
    echo "Error: .env file not found"
    exit 1
fi

# Verify TEST_KEY is set
if [ -z "${TEST_KEY:-}" ]; then
    echo "Error: TEST_KEY not found in .env"
    exit 1
fi

# Create logs directory if it doesn't exist
mkdir -p ./logs

# Set API key for server authentication
export API_KEYS="$TEST_KEY"

# Set logging configuration with timestamp
export LOG_FILE="$(pwd)/logs/reddit-server-$(date +%Y%m%d-%H%M%S).log"
export LOG_LEVEL="info"
export LOG_FORMAT="json"

# Run the server from the cmd/reddit-server directory
cd cmd/reddit-server
./reddit-server
