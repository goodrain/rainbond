#!/usr/bin/env python3
import argparse
import fcntl
import json
import sys
from pathlib import Path


def load_manifest(path):
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def save_manifest(path, manifest):
    with path.open("w", encoding="utf-8") as handle:
        json.dump(manifest, handle, indent=2, ensure_ascii=True)
        handle.write("\n")


def update_manifest(path, update_fn):
    with path.open("r+", encoding="utf-8") as handle:
        fcntl.flock(handle.fileno(), fcntl.LOCK_EX)
        manifest = json.load(handle)
        result = update_fn(manifest)
        handle.seek(0)
        json.dump(manifest, handle, indent=2, ensure_ascii=True)
        handle.write("\n")
        handle.truncate()
        fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
        return result


def render_markdown(manifest, output_path):
    lines = [
        "# 测试能力清单",
        "",
        "> 此文件由 `scripts/manage_test_manifest.py render` 自动生成，请勿手工编辑。",
        "",
        "| Capability ID | 中文标题 | 状态 | 测试类型 | 业务入口 | 测试文件 |",
        "|---|---|---|---|---|---|",
    ]

    for capability in sorted(manifest.get("capabilities", []), key=lambda item: item["id"]):
        tests = "<br>".join(
            [
                "{path}{selector}".format(
                    path=test.get("path", ""),
                    selector=("::" + test["selector"]) if test.get("selector") else "",
                )
                for test in capability.get("tests", [])
            ]
        )
        lines.append(
            "| {id} | {title_zh} | {status} | {test_type} | {interface} | {tests} |".format(
                id=capability.get("id", ""),
                title_zh=capability.get("title_zh") or capability.get("title", ""),
                status=capability.get("status", ""),
                test_type=capability.get("test_type", ""),
                interface=capability.get("interface", ""),
                tests=tests,
            )
        )

    lines.extend(["", "## 详情", ""])
    for capability in sorted(manifest.get("capabilities", []), key=lambda item: item["id"]):
        test_paths = [
            "{path}{selector}".format(
                path=test.get("path", ""),
                selector=("::" + test["selector"]) if test.get("selector") else "",
            )
            for test in capability.get("tests", [])
        ]
        lines.extend(
            [
                "### {0}".format(capability.get("title_zh") or capability.get("title", capability.get("id", ""))),
                "",
                "- Capability ID: `{0}`".format(capability.get("id", "")),
                "- 状态: `{0}`".format(capability.get("status", "")),
                "- 测试类型: `{0}`".format(capability.get("test_type", "")),
                "- 接口类型: `{0}`".format(capability.get("interface_type", "")),
                "- 业务入口: `{0}`".format(capability.get("interface", "")),
                "- 代码路径: `{0}`".format("`, `".join(capability.get("code_paths", []))),
                "- 测试路径: `{0}`".format("`, `".join(test_paths)),
                "",
            ]
        )

    output_path.write_text("\n".join(lines), encoding="utf-8")


def find_capability(manifest, capability_id):
    for index, capability in enumerate(manifest.get("capabilities", [])):
        if capability.get("id") == capability_id:
            return index, capability
    return None, None


def build_test_path_owners(manifest, ignore_capability_id=None):
    owners = {}
    for capability in manifest.get("capabilities", []):
        capability_id = capability.get("id")
        if capability_id == ignore_capability_id:
            continue
        for test_entry in capability.get("tests", []):
            test_path = test_entry.get("path")
            if test_path:
                owners.setdefault(test_path, set()).add(capability_id)
    return owners


def cmd_list(args):
    manifest = load_manifest(args.manifest)
    capabilities = manifest.get("capabilities", [])
    if not capabilities:
        print("no capabilities registered")
        return 0
    for capability in capabilities:
        print(
            "{id}\t{status}\t{test_type}\t{interface}".format(
                id=capability.get("id", ""),
                status=capability.get("status", ""),
                test_type=capability.get("test_type", ""),
                interface=capability.get("interface", ""),
            )
        )
    return 0


def cmd_show(args):
    manifest = load_manifest(args.manifest)
    _, capability = find_capability(manifest, args.capability_id)
    if capability is None:
        print("capability not found: {0}".format(args.capability_id), file=sys.stderr)
        return 1
    print(json.dumps(capability, indent=2, ensure_ascii=True))
    return 0


def cmd_render(args):
    manifest = load_manifest(args.manifest)
    render_markdown(manifest, args.output)
    print("rendered markdown: {0}".format(args.output))
    return 0


def parse_tests(test_args):
    tests = []
    for raw_test in test_args:
        if "::" in raw_test:
            path, selector = raw_test.split("::", 1)
            tests.append({"path": path, "selector": selector})
        else:
            tests.append({"path": raw_test})
    return tests


def cmd_add(args):
    def apply_update(manifest):
        _, capability = find_capability(manifest, args.capability_id)
        if capability is not None:
            raise ValueError("capability already exists: {0}".format(args.capability_id))

        entry = {
            "id": args.capability_id,
            "title": args.title,
            "title_zh": args.title_zh or args.title,
            "interface_type": args.interface_type,
            "interface": args.interface,
            "code_paths": list(args.code_path),
            "tests": parse_tests(args.test),
            "test_type": args.test_type,
            "status": args.status,
        }

        manifest.setdefault("capabilities", []).append(entry)
        manifest["capabilities"] = sorted(manifest["capabilities"], key=lambda item: item["id"])

    try:
        update_manifest(args.manifest, apply_update)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1
    render_markdown(load_manifest(args.manifest), args.output)
    print("added capability: {0}".format(args.capability_id))
    return 0


def cmd_prune(args):
    manifest = load_manifest(args.manifest)
    index, capability = find_capability(manifest, args.capability_id)
    if capability is None:
        print("capability not found: {0}".format(args.capability_id), file=sys.stderr)
        return 1

    repo_root = args.repo_root.resolve()
    owners = build_test_path_owners(manifest, ignore_capability_id=args.capability_id)
    unique_paths = []
    shared_paths = []

    for test_entry in capability.get("tests", []):
        test_path = test_entry.get("path")
        if not test_path:
            continue
        if owners.get(test_path):
            shared_paths.append(test_path)
        else:
            unique_paths.append(test_path)

    print("capability: {0}".format(args.capability_id))
    print("mode: {0}".format("drop" if args.drop else "retire"))
    print("delete uniquely owned files: {0}".format("yes" if args.delete_owned_test_files else "no"))
    print("dry run: {0}".format("yes" if args.dry_run else "no"))

    if unique_paths:
        print("uniquely owned test files:")
        for path in unique_paths:
            print("  - {0}".format(path))
    else:
        print("uniquely owned test files: none")

    if shared_paths:
        print("shared test files (manual cleanup required):")
        for path in shared_paths:
            print("  - {0}".format(path))
    else:
        print("shared test files: none")

    if not args.dry_run and args.delete_owned_test_files:
        for relative_path in unique_paths:
            full_path = repo_root / relative_path
            if full_path.exists():
                full_path.unlink()
                print("deleted test file: {0}".format(relative_path))

    if not args.dry_run:
        def apply_update(updated_manifest):
            updated_index, _ = find_capability(updated_manifest, args.capability_id)
            if updated_index is None:
                raise ValueError("capability not found: {0}".format(args.capability_id))
            if args.drop:
                updated_manifest["capabilities"].pop(updated_index)
            else:
                updated_manifest["capabilities"][updated_index]["status"] = "retired"

        try:
            update_manifest(args.manifest, apply_update)
        except ValueError as exc:
            print(str(exc), file=sys.stderr)
            return 1
        render_markdown(load_manifest(args.manifest), args.output)
        if args.drop:
            print("removed manifest entry: {0}".format(args.capability_id))
        else:
            print("retired manifest entry: {0}".format(args.capability_id))

    if shared_paths:
        print("note: shared test files were not modified")

    return 0


def build_parser():
    parser = argparse.ArgumentParser(description="Manage Rainbond test manifest entries.")
    parser.add_argument("--repo-root", type=Path, default=Path("."), help="Repository root")
    parser.add_argument("--manifest", type=Path, default=Path("test-manifest.json"), help="Manifest path")
    parser.add_argument("--output", type=Path, default=Path("test-manifest.md"), help="Readable manifest output path")

    subparsers = parser.add_subparsers(dest="command", required=True)

    list_parser = subparsers.add_parser("list", help="List all capabilities")
    list_parser.set_defaults(func=cmd_list)

    show_parser = subparsers.add_parser("show", help="Show one capability")
    show_parser.add_argument("capability_id")
    show_parser.set_defaults(func=cmd_show)

    render_parser = subparsers.add_parser("render", help="Render readable markdown")
    render_parser.set_defaults(func=cmd_render)

    add_parser = subparsers.add_parser("add", help="Add a capability entry")
    add_parser.add_argument("capability_id")
    add_parser.add_argument("--title", required=True)
    add_parser.add_argument("--title-zh")
    add_parser.add_argument("--interface-type", required=True)
    add_parser.add_argument("--interface", required=True)
    add_parser.add_argument("--code-path", action="append", required=True, default=[])
    add_parser.add_argument("--test", action="append", required=True, default=[])
    add_parser.add_argument("--test-type", required=True)
    add_parser.add_argument("--status", default="active")
    add_parser.set_defaults(func=cmd_add)

    prune_parser = subparsers.add_parser("prune", help="Retire or remove a capability and delete uniquely owned test files")
    prune_parser.add_argument("capability_id")
    prune_parser.add_argument("--drop", action="store_true", help="Remove the manifest entry instead of retiring it")
    prune_parser.add_argument(
        "--no-delete-owned-test-files",
        dest="delete_owned_test_files",
        action="store_false",
        help="Keep uniquely owned test files on disk",
    )
    prune_parser.add_argument("--dry-run", action="store_true", help="Report actions without changing files")
    prune_parser.set_defaults(func=cmd_prune, delete_owned_test_files=True)

    return parser


def main():
    parser = build_parser()
    args = parser.parse_args()
    args.repo_root = args.repo_root.resolve()
    args.manifest = args.manifest.resolve()
    if args.output.is_absolute():
        args.output = args.output.resolve()
    else:
        args.output = (args.repo_root / args.output).resolve()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
