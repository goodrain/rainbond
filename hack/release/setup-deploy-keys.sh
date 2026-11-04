#!/usr/bin/env bash
# setup-deploy-keys.sh — give release-publish write access to the protected main
# branches of the rainbond and chart repositories.
#
# release-publish pushes two commits: the chart version bump (chart repo) and the
# upgrade feed record (rainbond repo). Both main branches are protected by
# rulesets, so the workflow pushes with a per-repository deploy key and the
# rulesets let deploy keys bypass. GitHub does not allow one key on two
# repositories, so this script makes two.
#
# The private keys go straight from a temporary directory into the
# release-publish environment secrets; they are never printed and are deleted
# when the script exits. Run it as a repository admin with gh logged in:
#
#   bash hack/release/setup-deploy-keys.sh goodrain/rainbond goodrain/rainbond-chart
#
# Rotating: run it again with --replace; the old deploy keys are removed.
set -euo pipefail

replace=false
if [ "${1:-}" = --replace ]; then replace=true; shift; fi
if [ $# -ne 2 ]; then
  echo "usage: $0 [--replace] <rainbond repo> <chart repo>" >&2
  exit 2
fi
RAINBOND_REPO=$1
CHART_REPO=$2
ENVIRONMENT=release-publish
TITLE=release-publish

for r in "$RAINBOND_REPO" "$CHART_REPO"; do
  if [ "$(gh api "repos/$r" --jq .permissions.admin)" != true ]; then
    echo "you need admin on $r to add a deploy key" >&2
    exit 1
  fi
done
gh api "repos/$RAINBOND_REPO/environments/$ENVIRONMENT" >/dev/null 2>&1 || {
  echo "environment $ENVIRONMENT does not exist on $RAINBOND_REPO; create it first" >&2
  exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

add_key() {
  local repo=$1 secret=$2 ids
  ids=$(gh api "repos/$repo/keys" --jq ".[] | select(.title == \"$TITLE\") | .id")
  if [ -n "$ids" ]; then
    if [ "$replace" != true ]; then
      echo "$repo already has a deploy key titled $TITLE; rerun with --replace to rotate it" >&2
      exit 1
    fi
    for id in $ids; do gh api -X DELETE "repos/$repo/keys/$id"; done
  fi
  ssh-keygen -q -t ed25519 -N '' -C "$TITLE@$repo" -f "$tmp/$secret"
  gh repo deploy-key add "$tmp/$secret.pub" --repo "$repo" --allow-write --title "$TITLE" >/dev/null
  gh secret set "$secret" --repo "$RAINBOND_REPO" --env "$ENVIRONMENT" < "$tmp/$secret"
  echo "$repo: deploy key $(ssh-keygen -lf "$tmp/$secret.pub" | awk '{print $2}') -> secret $secret"
}

add_key "$RAINBOND_REPO" RAINBOND_DEPLOY_KEY
add_key "$CHART_REPO" CHART_DEPLOY_KEY

cat <<EOF

Done. One more step in the browser, on both repositories:
  Settings -> Rules -> Rulesets -> the ruleset protecting main -> Bypass list
  -> Add bypass -> Deploy keys -> Always allow -> Save
  https://github.com/$RAINBOND_REPO/settings/rules
  https://github.com/$CHART_REPO/settings/rules
EOF
