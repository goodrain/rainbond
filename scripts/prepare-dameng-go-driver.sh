#!/bin/sh

set -eu

usage() {
	cat <<'EOF'
Usage: prepare-dameng-go-driver.sh [bundle-directory]

Prepare the ignored local Dameng Go module used by a private DM image build.
The bundle directory must contain either the official combined dmgorm1.zip, or
the official Go driver archive together with gorm_v1_dialect.zip. The prepared
driver and GORM dialect modules are written below <bundle-directory> and are
never committed.
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
driver_module_dir="$bundle_dir/dm-driver"
dialect_module_dir="$bundle_dir/dm-dialect"
driver_module_path="github.com/goodrain/dameng-driver"

if [ ! -d "$bundle_dir" ]; then
	echo "Dameng Go driver bundle directory is missing" >&2
	exit 1
fi

if [ -f "$driver_module_dir/go.mod" ] && [ -f "$dialect_module_dir/go.mod" ] && \
    [ -f "$dialect_module_dir/dialect_dm.go" ]; then
	exit 0
fi

if [ -e "$driver_module_dir" ] || [ -e "$dialect_module_dir" ]; then
	echo "Dameng Go driver module directories are incomplete" >&2
	exit 1
fi

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
staging_driver_dir="$work_dir/dm-driver"
staging_dialect_dir="$work_dir/dm-dialect"

combined_archive=$(find "$bundle_dir" -maxdepth 1 -type f -name 'dmgorm1.zip' -print -quit)
driver_archive=$(find "$bundle_dir" -maxdepth 1 -type f \( -name 'dm-go-driver.zip' -o -name 'dm.zip' \) -print -quit)
dialect_archive=$(find "$bundle_dir" -maxdepth 1 -type f -name 'gorm_v1_dialect.zip' -print -quit)

mkdir -p "$staging_driver_dir" "$staging_dialect_dir"

if [ -n "$combined_archive" ]; then
	unzip -qq "$combined_archive" -d "$work_dir/combined"
	dialect_file=$(find "$work_dir/combined" -type f -name 'dialect_dm.go' -print -quit)
	if [ -z "$dialect_file" ]; then
		echo "Dameng combined Go bundle does not contain the GORM v1 dialect" >&2
		exit 1
	fi
	driver_mod=$(find "$work_dir/combined" -type f -name 'go.mod' -print -quit)
	if [ -z "$driver_mod" ]; then
		echo "Dameng combined Go bundle does not contain a Go module" >&2
		exit 1
	fi
	cp -R "$(dirname "$driver_mod")"/. "$staging_driver_dir"
elif [ -n "$driver_archive" ] && [ -n "$dialect_archive" ]; then
	unzip -qq "$driver_archive" -d "$work_dir/driver"
	driver_mod=$(find "$work_dir/driver" -type f -name 'go.mod' -print -quit)
	if [ -z "$driver_mod" ]; then
		echo "Dameng Go driver archive does not contain a Go module" >&2
		exit 1
	fi
	cp -R "$(dirname "$driver_mod")"/. "$staging_driver_dir"

	unzip -qq "$dialect_archive" -d "$work_dir/dialect"
	dialect_file=$(find "$work_dir/dialect" -type f -name 'dialect_dm.go' -print -quit)
	if [ -z "$dialect_file" ]; then
		echo "Dameng GORM v1 dialect archive is incomplete" >&2
		exit 1
	fi
else
	echo "Dameng Go driver bundle must include dmgorm1.zip or dm-go-driver.zip with gorm_v1_dialect.zip" >&2
	exit 1
fi

if [ ! -f "$staging_driver_dir/go.mod" ]; then
	echo "Dameng Go driver module is missing go.mod" >&2
	exit 1
fi

if ! grep -q '^module dm$' "$staging_driver_dir/go.mod"; then
	echo "Dameng Go driver module path must be dm" >&2
	exit 1
fi

rm -f "$staging_driver_dir/dialect_dm.go"
cp "$dialect_file" "$staging_dialect_dir/dialect_dm.go"
LC_ALL=C sed "s#^module dm\$#module ${driver_module_path}#" "$staging_driver_dir/go.mod" > "$staging_driver_dir/go.mod.new"
mv "$staging_driver_dir/go.mod.new" "$staging_driver_dir/go.mod"
find "$staging_driver_dir" -type f -name '*.go' -exec sh -c '
	file=$1
	LC_ALL=C sed "s#\"dm/#\"github.com/goodrain/dameng-driver/#g" "$file" > "$file.new"
	mv "$file.new" "$file"
' sh {} \;
LC_ALL=C sed "s#\"dm\"#\"${driver_module_path}\"#" "$staging_dialect_dir/dialect_dm.go" > "$staging_dialect_dir/dialect_dm.go.new"
mv "$staging_dialect_dir/dialect_dm.go.new" "$staging_dialect_dir/dialect_dm.go"
cat > "$staging_dialect_dir/go.mod" <<'EOF'
module github.com/goodrain/dameng-gorm-dialect

go 1.13

require (
	github.com/goodrain/dameng-driver v0.0.0
	github.com/jinzhu/gorm v1.9.16
)

replace github.com/goodrain/dameng-driver => ../dm-driver
EOF

mv "$staging_driver_dir" "$driver_module_dir"
mv "$staging_dialect_dir" "$dialect_module_dir"
