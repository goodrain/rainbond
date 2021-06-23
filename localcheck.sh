#!/bin/sh
TARGET_BRANCH=${TARGET_BRANCH:-'master'}

CHANGED_MARKDOWN_FILES=$(git diff --name-only ${TARGET_BRANCH} | grep .go)
for file in ${CHANGED_MARKDOWN_FILES}; do
  echo "golint ${file}"
  golint -set_exit_status=true ${file} || exit 1
done

echo "code golint check success"