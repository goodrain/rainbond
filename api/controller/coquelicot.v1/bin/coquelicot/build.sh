#!/bin/sh

# Writes a version file with the latest git commit id
# and any tag associated with it.

COMMIT=$(git log --format="%h" -n 1)
TAG=$(git describe --all --exact-match $COMMIT)

cat > version.go << EOF
package main

const (
    appVersion = "$COMMIT $TAG"
)
EOF

go build && rm -f version.go
