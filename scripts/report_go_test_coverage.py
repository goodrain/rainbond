#!/usr/bin/env python3
import argparse
import os
import shutil
import subprocess
import tempfile
from pathlib import Path


DEFAULT_ROOTS = ["api", "builder", "db", "event", "pkg", "util", "worker"]
DEFAULT_EXCLUDES = {"worker/master/controller/helmapp"}


def parse_args():
    parser = argparse.ArgumentParser(description="Aggregate Go coverage across tested packages.")
    parser.add_argument("--timeout", type=int, default=60, help="Per-package timeout in seconds")
    parser.add_argument("--worst", type=int, default=15, help="How many lowest-coverage packages to print")
    parser.add_argument(
        "--exclude",
        action="append",
        default=[],
        help="Relative package directory to exclude from the aggregate run",
    )
    return parser.parse_args()


def package_coverage(profile_path):
    statements = 0
    covered = 0
    with profile_path.open("r", encoding="utf-8") as handle:
        for index, line in enumerate(handle):
            if index == 0:
                continue
            _, metrics = line.strip().split(" ", 1)
            stmt_count, hit_count = metrics.split(" ")
            stmt_count = int(stmt_count)
            hit_count = int(hit_count)
            statements += stmt_count
            if hit_count > 0:
                covered += stmt_count
    return statements, covered


def discover_packages(repo_root, excludes):
    packages = []
    for root_name in DEFAULT_ROOTS:
        root = repo_root / root_name
        if not root.exists():
            continue
        for test_file in root.rglob("*_test.go"):
            relative_dir = test_file.parent.relative_to(repo_root).as_posix()
            if relative_dir.startswith("util/envoy/"):
                continue
            if relative_dir in excludes:
                continue
            packages.append(relative_dir)
    return sorted(set(packages))


def main():
    args = parse_args()
    repo_root = Path(__file__).resolve().parents[1]
    excludes = set(DEFAULT_EXCLUDES)
    excludes.update(args.exclude)

    packages = discover_packages(repo_root, excludes)
    if not packages:
        print("no test packages discovered")
        return 1

    env = os.environ.copy()
    env.setdefault("GOCACHE", "/tmp/rainbond-go-build-cache")

    with tempfile.TemporaryDirectory(prefix="rainbond-go-cover-") as tempdir:
        temp_root = Path(tempdir)
        aggregate_lines = []
        mode_line = None
        successes = []
        failures = []

        for package_dir in packages:
            profile = temp_root / (package_dir.replace("/", "_") + ".cover")
            cmd = [
                "/Users/zhangqihang/go/bin/go1.24.6",
                "test",
                "./" + package_dir,
                "-coverprofile={0}".format(profile),
            ]
            try:
                proc = subprocess.run(
                cmd,
                cwd=str(repo_root),
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                universal_newlines=True,
                timeout=args.timeout,
            )
            except subprocess.TimeoutExpired:
                failures.append((package_dir, "timeout", []))
                continue

            if proc.returncode != 0 or not profile.exists():
                tail = proc.stdout.strip().splitlines()[-5:]
                failures.append((package_dir, "exit={0}".format(proc.returncode), tail))
                continue

            lines = profile.read_text(encoding="utf-8").splitlines()
            if not lines:
                failures.append((package_dir, "empty-profile", []))
                continue

            if mode_line is None:
                mode_line = lines[0]
            aggregate_lines.extend(lines[1:])

            statements, covered = package_coverage(profile)
            percent = round(covered / statements * 100, 2) if statements else 0.0
            successes.append((package_dir, percent, statements, covered))

        print("scope: tested Go packages")
        print("discovered_packages: {0}".format(len(packages)))
        print("passed_packages: {0}".format(len(successes)))
        print("failed_packages: {0}".format(len(failures)))

        if not successes or mode_line is None:
            print("no successful package coverage profiles generated")
            for package_dir, reason, tail in failures:
                print("FAIL {0} {1}".format(package_dir, reason))
                for line in tail:
                    print("  {0}".format(line))
            return 1

        aggregate_profile = temp_root / "aggregate.cover"
        aggregate_profile.write_text(mode_line + "\n" + "\n".join(aggregate_lines) + "\n", encoding="utf-8")
        total_statements, total_covered = package_coverage(aggregate_profile)
        total_percent = round(total_covered / total_statements * 100, 2) if total_statements else 0.0
        print("weighted_coverage: {0}%".format(total_percent))
        print("lowest_coverage_packages:")
        for package_dir, percent, statements, _ in sorted(successes, key=lambda item: (item[1], -item[2], item[0]))[: args.worst]:
            print("  {0:>6}%  {1:>5}  {2}".format(percent, statements, package_dir))

        if failures:
            print("failed_packages_detail:")
            for package_dir, reason, tail in failures:
                print("  {0}  {1}".format(package_dir, reason))
                for line in tail:
                    print("    {0}".format(line))
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
