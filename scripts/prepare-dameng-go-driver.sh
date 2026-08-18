#!/bin/sh

set -eu

usage() {
	cat <<'EOF'
Usage: prepare-dameng-go-driver.sh [bundle-directory]

Prepare the ignored local Dameng Go module used by a private DM image build.
The bundle directory must contain either the official combined dmgorm1.zip, or
the official Go driver archive together with gorm_v1_dialect.zip. The prepared
module is written to <bundle-directory>/dm and is never committed.
EOF
}

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
	usage
	exit 0
fi

if [ "$#" -gt 1 ]; then
	usage >&2
	exit 2
fi

bundle_dir=${1:-third_party/dameng}
module_dir="$bundle_dir/dm"

if [ ! -d "$bundle_dir" ]; then
	echo "Dameng Go driver bundle directory is missing" >&2
	exit 1
fi

if [ -f "$module_dir/go.mod" ] && [ -f "$module_dir/dialect_dm.go" ]; then
	exit 0
fi

if [ -e "$module_dir" ]; then
	echo "Dameng Go driver module directory is incomplete" >&2
	exit 1
fi

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
staging_dir="$work_dir/dm"

combined_archive=$(find "$bundle_dir" -maxdepth 1 -type f -name 'dmgorm1.zip' -print -quit)
driver_archive=$(find "$bundle_dir" -maxdepth 1 -type f \( -name 'dm-go-driver.zip' -o -name 'dm.zip' \) -print -quit)
dialect_archive=$(find "$bundle_dir" -maxdepth 1 -type f -name 'gorm_v1_dialect.zip' -print -quit)

mkdir -p "$staging_dir"

if [ -n "$combined_archive" ]; then
	unzip -qq "$combined_archive" -d "$work_dir/combined"
	dialect_file=$(find "$work_dir/combined" -type f -name 'dialect_dm.go' -print -quit)
	if [ -z "$dialect_file" ]; then
		echo "Dameng combined Go bundle does not contain the GORM v1 dialect" >&2
		exit 1
	fi
	cp -R "$(dirname "$dialect_file")"/. "$staging_dir"
elif [ -n "$driver_archive" ] && [ -n "$dialect_archive" ]; then
	unzip -qq "$driver_archive" -d "$work_dir/driver"
	driver_mod=$(find "$work_dir/driver" -type f -name 'go.mod' -print -quit)
	if [ -z "$driver_mod" ]; then
		echo "Dameng Go driver archive does not contain a Go module" >&2
		exit 1
	fi
	cp -R "$(dirname "$driver_mod")"/. "$staging_dir"

	unzip -qq "$dialect_archive" -d "$work_dir/dialect"
	dialect_file=$(find "$work_dir/dialect" -type f -name 'dialect_dm.go' -print -quit)
	if [ -z "$dialect_file" ]; then
		echo "Dameng GORM v1 dialect archive is incomplete" >&2
		exit 1
	fi
	cp "$dialect_file" "$staging_dir/dialect_dm.go"
else
	echo "Dameng Go driver bundle must include dmgorm1.zip or dm-go-driver.zip with gorm_v1_dialect.zip" >&2
	exit 1
fi

if [ ! -f "$staging_dir/go.mod" ]; then
	echo "Dameng Go driver module is missing go.mod" >&2
	exit 1
fi

if ! grep -q '^module dm$' "$staging_dir/go.mod"; then
	echo "Dameng Go driver module path must be dm" >&2
	exit 1
fi

mv "$staging_dir" "$module_dir"
