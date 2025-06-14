#!/bin/bash

# Check if exactly 2 arguments are provided
if [ $# -ne 2 ]; then
    echo "Usage: $0 <subject> <description>"
    echo "Example: $0 \"Add SQLite3 support\" \"Implemented RocksDB-optimized SQLite3 backend\""
    exit 1
fi

SUBJECT="$1"
DESCRIPTION="$2"
TIMESTAMP=$(date -u +%s)

echo "Committing at UTC timestamp: $TIMESTAMP"
echo "Subject: $SUBJECT"
echo "Description: $DESCRIPTION"

# Add all changes
git add .

# Commit with subject and description
git commit -m "$SUBJECT" -m "$DESCRIPTION"

# Check if commit was successful
if [ $? -eq 0 ]; then
    echo "Commit successful. Pushing to remote..."
    git push
    if [ $? -eq 0 ]; then
        echo "Push successful!"
    else
        echo "Push failed!"
        exit 1
    fi
else
    echo "Commit failed!"
    exit 1
fi
