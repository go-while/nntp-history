#!/bin/bash

# Check if exactly 2 arguments are provided
#if [ $# -ne 2 ]; then
#    echo "Usage: $0 <subject> <description>"
#    echo "Example: $0 \"Fix bug in parser\" \"Resolved issue with malformed headers causing crashes\""
#    exit 1
#fi

SUBJECT="$1"
DESCRIPTION="$2"

now=$(date -u +%s)

test -z "$SUBJECT" && SUBJECT="patch-$now"
test -z "$DESCRIPTION" && DESCRIPTION="patch-$now"

# Add all changes
git add .

# Commit with subject and description
COMMIT=$(git commit -m "$SUBJECT" -m "$DESCRIPTION")

# Check if commit was successful
if [ $? -eq 0 ]; then
    echo "Commit successful. Pushing to remote..."
    git push
    if [ $? -eq 0 ]; then
        echo "Push successful!"
	echo "$COMMIT"
	HASH=$(echo "$COMMIT"|head -1|cut -d"[" -f2|cut -d"]" -f1|cut -d" " -f2)
	cd ../nntp-history_test && ./go-get.sh "$HASH"
    else
        echo "Push failed!"
        exit 1
    fi
else
    echo "Commit failed!"
    exit 1
fi
