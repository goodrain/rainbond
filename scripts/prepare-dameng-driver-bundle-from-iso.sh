#!/usr/bin/env bash
# Extract the smallest DM driver build context from an official DM8 Linux ISO.
set -euo pipefail

usage() {
    cat <<'EOF'
Usage: scripts/prepare-dameng-driver-bundle-from-iso.sh <dm8-iso> [output-directory]

Extracts only the Go driver archives, dmPython, dmDjango3.0, and the DPI runtime
materials from an official DM8 Linux ISO. The output is a Docker build context:

  go/dm-go-driver.zip
  go/gorm_v1_dialect.zip
  python/dpi/{libdmdpi.so,dependencies/,include/}
  python/drivers/python/{dmPython/,dmDjango/dmDjango3.0/}

The ISO and its complete DM installation tree are never copied to the output.
EOF
}

if [ "$#" -eq 1 ] && [ "$1" = "--help" ]; then
    usage
    exit 0
fi

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    usage >&2
    exit 2
fi

iso_path=$1
project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
output_root=${2:-"${project_root}/third_party/dameng"}

if [ ! -f "${iso_path}" ]; then
    echo "DM8 ISO does not exist: ${iso_path}" >&2
    exit 1
fi

if ! command -v bsdtar >/dev/null 2>&1; then
    echo "bsdtar is required to read the DM8 ISO" >&2
    exit 1
fi

if ! bsdtar -tf "${iso_path}" | grep -Fxq "DMInstall.bin"; then
    echo "DM8 ISO does not contain DMInstall.bin" >&2
    exit 1
fi

if [ -e "${output_root}" ]; then
    echo "destination already exists: ${output_root}" >&2
    exit 1
fi

output_parent=$(dirname "${output_root}")
mkdir -p "${output_parent}"
staging_root=$(mktemp -d "${output_parent}/.dameng-iso.XXXXXX")
trap 'rm -rf "${staging_root}"' EXIT

installer_path="${staging_root}/DMInstall.bin"
bsdtar -xOf "${iso_path}" DMInstall.bin > "${installer_path}"
skip=$(LC_ALL=C sed -n 's/^skip=\([0-9][0-9]*\)$/\1/p' "${installer_path}" | head -n 1)
case "${skip}" in
    ''|*[!0-9]*)
        echo "DMInstall.bin does not declare a valid payload offset" >&2
        exit 1
        ;;
esac

payload_root="${staging_root}/payload"
mkdir -p "${payload_root}"
set +o pipefail
tail -n +"${skip}" "${installer_path}" | tar -xzf - -C "${payload_root}" \
    source/drivers/go/dm-go-driver.zip \
    source/drivers/go/gorm_v1_dialect.zip \
    source/drivers/dpi/libdmdpi.so \
    source/drivers/dpi/dependencies \
    source/drivers/dpi/include \
    source/drivers/python/dmPython \
    source/drivers/python/dmDjango/dmDjango3.0
tar_status=${PIPESTATUS[1]}
set -o pipefail
if [ "${tar_status}" -ne 0 ]; then
    echo "failed to extract required driver material from DMInstall.bin" >&2
    exit 1
fi

required_paths=(
    "${payload_root}/source/drivers/go/dm-go-driver.zip"
    "${payload_root}/source/drivers/go/gorm_v1_dialect.zip"
    "${payload_root}/source/drivers/dpi/libdmdpi.so"
    "${payload_root}/source/drivers/dpi/dependencies"
    "${payload_root}/source/drivers/dpi/include"
    "${payload_root}/source/drivers/python/dmPython"
    "${payload_root}/source/drivers/python/dmDjango/dmDjango3.0"
)
for required_path in "${required_paths[@]}"; do
    if [ ! -e "${required_path}" ]; then
        echo "DM8 ISO is missing required driver material: ${required_path#"${payload_root}/source/"}" >&2
        exit 1
    fi
done

bundle_root="${staging_root}/bundle"
mkdir -p "${bundle_root}/go" \
    "${bundle_root}/python/dpi/dependencies" \
    "${bundle_root}/python/dpi/include" \
    "${bundle_root}/python/drivers/python/dmPython" \
    "${bundle_root}/python/drivers/python/dmDjango/dmDjango3.0"

cp "${payload_root}/source/drivers/go/dm-go-driver.zip" "${bundle_root}/go/"
cp "${payload_root}/source/drivers/go/gorm_v1_dialect.zip" "${bundle_root}/go/"
cp "${payload_root}/source/drivers/dpi/libdmdpi.so" "${bundle_root}/python/dpi/"
cp -R "${payload_root}/source/drivers/dpi/dependencies/." "${bundle_root}/python/dpi/dependencies/"
cp -R "${payload_root}/source/drivers/dpi/include/." "${bundle_root}/python/dpi/include/"
cp -R "${payload_root}/source/drivers/python/dmPython/." "${bundle_root}/python/drivers/python/dmPython/"
cp -R "${payload_root}/source/drivers/python/dmDjango/dmDjango3.0/." \
    "${bundle_root}/python/drivers/python/dmDjango/dmDjango3.0/"

mv "${bundle_root}" "${output_root}"
trap - EXIT
rm -rf "${staging_root}"
echo "Dameng driver bundle prepared at ${output_root}"
