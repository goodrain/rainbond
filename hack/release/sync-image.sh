#!/usr/bin/env bash
# Copy existing compressed layers; never rebuild or recompress the image.
set -euo pipefail

copy_mode=(--all)
if [[ "${1:-}" == "--index-only" ]]; then
  copy_mode=(--multi-arch=index-only)
  shift
fi
source_image=$1
target_image=$2
expected_digest=${source_image##*@}
[[ "$expected_digest" =~ ^sha256:[a-f0-9]{64}$ ]] || { echo "::error::Source must use an image digest"; exit 1; }

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cache_dir=$(mktemp -d "${RUNNER_TEMP:?}/registry-copy.XXXXXX")
trap 'rm -rf "$cache_dir"' EXIT
auth_file="${DOCKER_CONFIG:-$HOME/.docker}/config.json"

# Skopeo uses GET token authentication with Docker's stored username/password.
# Keep downloaded blobs on disk so an ACR retry cannot repeat the Hub download
# or attempt a cross-registry blob mount.
echo "Downloading ${source_image} to the runner"
bash "$script_dir/retry.sh" skopeo copy "${copy_mode[@]}" --preserve-digests --authfile "$auth_file" \
  "docker://${source_image}" "dir:${cache_dir}/image"

echo "Uploading cached layers to ${target_image}"
bash "$script_dir/retry.sh" skopeo copy "${copy_mode[@]}" --preserve-digests --authfile "$auth_file" \
  --digestfile "${cache_dir}/uploaded-digest" "dir:${cache_dir}/image" "docker://${target_image}"

[[ "$(cat "${cache_dir}/uploaded-digest")" == "$expected_digest" ]] || {
  echo "::error::Synced image digest differs from the build output"
  exit 1
}
echo "Verified ${target_image}@${expected_digest}"
