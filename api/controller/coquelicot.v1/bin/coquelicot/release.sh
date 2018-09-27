#!/bin/sh

# Writes a version file with the latest git commit id
# and any tag associated with it.

COMMIT=$(git log --format="%h" -n 1)
TAG=$(git describe --all --exact-match $COMMIT)
BIN=coquelicot

cat > version.go << EOF
package main

const (
    appVersion = "$COMMIT $TAG"
)
EOF

echo "(use -a to build for all platforms (go >= 1.5)"

ARCH=amd64
OSLIST=$(uname -s|tr '[:upper:]' '[:lower:]')

if [ "$1" = "-a" ]; then
    OSLIST="linux windows freebsd darwin"
fi

rm -f /tmp/$BIN*.zip

RTAG=$(git tag|tail -1)
if [ -z "$RTAG" ]; then
    RTAG=$COMMIT
fi

for OS in $OSLIST; do
    echo "Building for $OS ..."
    TMP=/tmp/c$OS$ARCH
    rm -rf $TMP
    GOOS=$OS GOARCH=$ARCH go build -o $TMP/$BIN
    if [ "$OS" = "windows" ]; then
        mv $TMP/$BIN $TMP/$BIN.exe
    fi
    zip -j /tmp/$BIN-$RTAG-$OS-$ARCH.zip $TMP/$BIN* && rm -rf $TMP
done

rm -f version.go
