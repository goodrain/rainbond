#!/bin/sh

STAGED_GO_FILES=$(git diff --cached --name-only | grep ".go$")

if [[ "$STAGED_GO_FILES" = "" ]]; then
  exit 0
fi

PASS=true

for FILE in $STAGED_GO_FILES
do
  goimports -w $FILE
  golangci-lint run --fix --disable=typecheck $FILE
  if [[ $? != 0 ]]; then
    PASS=false
  fi
done

if ! $PASS; then
  printf "COMMIT FAILED\n"
  exit 1
else
  printf "COMMIT SUCCEEDED\n"
  git add
fi

exit 0
