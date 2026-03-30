#!/usr/bin/env python3
import argparse
import json
import re
import sys
from pathlib import Path


CAPABILITY_RE = re.compile(r"capability_id\s*:\s*([A-Za-z0-9_.-]+)")
PYTHON_TEST_PATTERNS = ("test_*.py", "*_test.py", "*_tests.py", "tests.py")
GO_TEST_PATTERNS = ("*_test.go",)
ALLOWED_INTERFACE_TYPES = {
    "service_method",
    "view_endpoint",
    "handler_method",
    "dao_method",
    "package_function",
    "workflow",
    "other",
}
ALLOWED_TEST_TYPES = {"unit", "regression", "characterization", "integration"}
ALLOWED_STATUSES = {"draft", "active", "retired"}


def load_manifest(path):
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def read_text(path):
    try:
        return path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return path.read_text(encoding="utf-8", errors="ignore")


def collect_marked_tests(repo_root):
    marked = {}
    patterns = list(PYTHON_TEST_PATTERNS) + list(GO_TEST_PATTERNS)
    for pattern in patterns:
        for file_path in repo_root.rglob(pattern):
            if any(part in {".git", "vendor", "_output", "node_modules", "dist"} for part in file_path.parts):
                continue
            matches = CAPABILITY_RE.findall(read_text(file_path))
            if matches:
                marked[str(file_path.relative_to(repo_root))] = sorted(set(matches))
    return marked


def ensure(condition, message, errors):
    if not condition:
        errors.append(message)


def validate_manifest(repo_root, manifest_path):
    errors = []
    manifest = load_manifest(manifest_path)

    ensure(isinstance(manifest, dict), "manifest root must be an object", errors)
    ensure(manifest.get("version") == 1, "manifest version must be 1", errors)
    ensure(isinstance(manifest.get("repo"), str) and manifest["repo"], "manifest repo must be a non-empty string", errors)

    capabilities = manifest.get("capabilities")
    ensure(isinstance(capabilities, list), "manifest capabilities must be an array", errors)
    if not isinstance(capabilities, list):
        return errors

    seen_ids = set()
    capability_test_paths = {}

    for index, capability in enumerate(capabilities):
        prefix = "capabilities[{0}]".format(index)
        ensure(isinstance(capability, dict), prefix + " must be an object", errors)
        if not isinstance(capability, dict):
            continue

        capability_id = capability.get("id")
        title = capability.get("title")
        interface_type = capability.get("interface_type")
        interface = capability.get("interface")
        code_paths = capability.get("code_paths")
        tests = capability.get("tests")
        test_type = capability.get("test_type")
        status = capability.get("status")

        ensure(isinstance(capability_id, str) and capability_id, prefix + ".id must be a non-empty string", errors)
        ensure(capability_id not in seen_ids, prefix + ".id must be unique", errors)
        if isinstance(capability_id, str) and capability_id:
            seen_ids.add(capability_id)
        ensure(isinstance(title, str) and title, prefix + ".title must be a non-empty string", errors)
        ensure(interface_type in ALLOWED_INTERFACE_TYPES, prefix + ".interface_type is invalid", errors)
        ensure(isinstance(interface, str) and interface, prefix + ".interface must be a non-empty string", errors)
        ensure(isinstance(code_paths, list), prefix + ".code_paths must be an array", errors)
        ensure(isinstance(tests, list), prefix + ".tests must be an array", errors)
        ensure(test_type in ALLOWED_TEST_TYPES, prefix + ".test_type is invalid", errors)
        ensure(status in ALLOWED_STATUSES, prefix + ".status is invalid", errors)

        normalized_test_paths = set()

        if isinstance(code_paths, list) and status in {"draft", "active"}:
            for code_path in code_paths:
                ensure(isinstance(code_path, str) and code_path, prefix + ".code_paths entries must be non-empty strings", errors)
                if isinstance(code_path, str) and code_path:
                    ensure((repo_root / code_path).exists(), prefix + ".code_paths missing: " + code_path, errors)

        if isinstance(tests, list):
            if status == "active":
                ensure(len(tests) > 0, prefix + ".tests must not be empty for active capabilities", errors)
            for test_index, test_entry in enumerate(tests):
                test_prefix = "{0}.tests[{1}]".format(prefix, test_index)
                ensure(isinstance(test_entry, dict), test_prefix + " must be an object", errors)
                if not isinstance(test_entry, dict):
                    continue
                test_path = test_entry.get("path")
                selector = test_entry.get("selector")
                ensure(isinstance(test_path, str) and test_path, test_prefix + ".path must be a non-empty string", errors)
                if selector is not None:
                    ensure(isinstance(selector, str) and selector, test_prefix + ".selector must be a non-empty string when present", errors)
                if isinstance(test_path, str) and test_path:
                    normalized_test_paths.add(test_path)
                    if status in {"draft", "active"}:
                        ensure((repo_root / test_path).exists(), test_prefix + ".path missing: " + test_path, errors)

        if isinstance(capability_id, str) and capability_id:
            capability_test_paths[capability_id] = normalized_test_paths

    marked = collect_marked_tests(repo_root)
    manifest_ids = set(capability_test_paths.keys())

    for test_path, capability_ids in marked.items():
        for capability_id in capability_ids:
            ensure(capability_id in manifest_ids, "managed test references unknown capability_id: {0} in {1}".format(capability_id, test_path), errors)
            if capability_id in capability_test_paths:
                ensure(
                    test_path in capability_test_paths[capability_id],
                    "managed test {0} must be listed under capability {1}".format(test_path, capability_id),
                    errors,
                )

    for capability_id, test_paths in capability_test_paths.items():
        for test_path in test_paths:
            if test_path in marked:
                ensure(
                    capability_id in marked[test_path],
                    "manifest lists {0} for capability {1} but test file marker is missing".format(test_path, capability_id),
                    errors,
                )

    return errors


def main():
    parser = argparse.ArgumentParser(description="Validate Rainbond test manifest consistency.")
    parser.add_argument("--repo-root", default=".", help="Repository root")
    parser.add_argument("--manifest", default="test-manifest.json", help="Manifest path relative to repo root")
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    manifest_path = (repo_root / args.manifest).resolve()

    if not manifest_path.exists():
        print("manifest not found: {0}".format(manifest_path), file=sys.stderr)
        return 1

    try:
        errors = validate_manifest(repo_root, manifest_path)
    except json.JSONDecodeError as exc:
        print("invalid manifest json: {0}".format(exc), file=sys.stderr)
        return 1

    if errors:
        for error in errors:
            print("ERROR: {0}".format(error), file=sys.stderr)
        return 1

    print("test manifest validation passed for {0}".format(repo_root.name))
    return 0


if __name__ == "__main__":
    sys.exit(main())

