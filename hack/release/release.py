#!/usr/bin/env python3
"""Release publishing helpers for Rainbond.

Every step that used to be done by hand after a GitHub release is published
lives here as a small, testable subcommand, so the release-publish workflow
only wires them together:

  check-tag        validate a release tag and print derived versions
  verify-images    make sure every release image is multi-arch (amd64+arm64)
  upgrade-versions add the new release to upgrade-versions.json
  install-script   stamp the default version into the quick install script
  chart-bump       set Chart.yaml version and values.yaml installVersion
  docs-versions    roll the docs version table when the minor version changes
  verify-url       compare a published URL with the local artifact
  check-e2e        make sure an E2E provenance covers the commits about to ship

Only the Python standard library is used so the script runs on a bare runner.
"""

import argparse
import datetime
import hashlib
import json
import re
import sys
import time
import urllib.error
import urllib.request

TAG_RE = re.compile(r"v(\d+)\.(\d+)\.(\d+)-release")

# Every image the upgrade feed points at must come from the official registry
# namespace. The feed is read by every Rainbond console that checks for
# upgrades, so this is a supply-chain guard, not a style check.
ALLOWED_IMAGE_PREFIX = "registry.cn-hangzhou.aliyuncs.com/goodrain/"

# Images produced by release-v6.yml with build-who=all. rainbond-ui is only
# pushed to Docker Hub (see the multi-arch-manifest job).
RELEASE_IMAGES = [
    "docker.io/rainbond/rbd-api",
    "docker.io/rainbond/rbd-chaos",
    "docker.io/rainbond/rbd-worker",
    "docker.io/rainbond/rbd-init-probe",
    "docker.io/rainbond/rbd-mq",
    "docker.io/rainbond/rainbond-operator",
    "docker.io/rainbond/rainbond",
    "docker.io/rainbond/rainbond-ui",
    "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-api",
    "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-chaos",
    "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-worker",
    "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-init-probe",
    "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-mq",
    "registry.cn-hangzhou.aliyuncs.com/goodrain/rainbond-operator",
    "registry.cn-hangzhou.aliyuncs.com/goodrain/rainbond",
]
REQUIRED_ARCHES = {"amd64", "arm64"}

# Components the console upgrades in place, and the repository each one is
# published under in the official namespace.
UPGRADE_COMPONENTS = {
    "rbd-api": "rbd-api",
    "rbd-app-ui": "rainbond",
    "rbd-chaos": "rbd-chaos",
    "rbd-mq": "rbd-mq",
    "rbd-worker": "rbd-worker",
}

INSTALL_VERSION_RE = re.compile(r"^RAINBOND_VERSION=\$\{VERSION:-'[^']*'\}$", re.M)
CHART_VERSION_RE = re.compile(r"^version:[ \t]*\S+[ \t]*$", re.M)
VALUES_INSTALL_VERSION_RE = re.compile(r"^([ \t]*)installVersion:[ \t]*\S+[ \t]*$", re.M)

MANIFEST_ACCEPT = ", ".join([
    "application/vnd.oci.image.index.v1+json",
    "application/vnd.docker.distribution.manifest.list.v2+json",
    "application/vnd.oci.image.manifest.v1+json",
    "application/vnd.docker.distribution.manifest.v2+json",
])


class ReleaseError(Exception):
    """A release precondition is not met; the message is shown to the operator."""


# --------------------------------------------------------------------------
# tags
# --------------------------------------------------------------------------

def parse_tag(tag):
    m = TAG_RE.fullmatch(tag)
    if not m:
        raise ReleaseError(f"tag {tag!r} is not a release tag like v6.9.12-release")
    return tuple(int(x) for x in m.groups())


def chart_version(tag):
    return "{}.{}.{}".format(*parse_tag(tag))


# --------------------------------------------------------------------------
# upgrade-versions.json
# --------------------------------------------------------------------------

def dump_upgrade_versions(entries):
    return json.dumps(entries, indent=2, ensure_ascii=False) + "\n"


def validate_upgrade_versions(entries):
    seen = set()
    for e in entries:
        version = e.get("version", "")
        parse_tag(version)
        if version in seen:
            raise ReleaseError(f"duplicate version {version} in upgrade-versions.json")
        seen.add(version)
        for name, image in e.get("images", {}).items():
            if not image.startswith(ALLOWED_IMAGE_PREFIX):
                raise ReleaseError(
                    f"{version}: image {name}={image} is outside {ALLOWED_IMAGE_PREFIX}")
    versions = [parse_tag(e["version"]) for e in entries]
    if versions != sorted(versions, reverse=True):
        raise ReleaseError("upgrade-versions.json must be sorted newest first")


def upgrade_entry(tag, release_date):
    parse_tag(tag)
    datetime.date.fromisoformat(release_date)
    url = f"https://github.com/goodrain/rainbond/releases/tag/{tag}"
    return {
        "version": tag,
        "detail": {
            "update_time": release_date,
            "html_url": url,
            "tag_name": tag,
            "body": f"For update details, reference: {url}.",
        },
        "images": {
            name: f"{ALLOWED_IMAGE_PREFIX}{repo}:{tag}"
            for name, repo in sorted(UPGRADE_COMPONENTS.items())
        },
    }


def add_upgrade_version(entries, tag, release_date):
    """Return entries with tag added; re-running for the same tag is a no-op
    as long as the existing entry is identical."""
    new = upgrade_entry(tag, release_date)
    for e in entries:
        if e["version"] == tag:
            if e != new:
                raise ReleaseError(
                    f"{tag} already exists in upgrade-versions.json with different "
                    "content; fix it by hand instead of overwriting")
            return list(entries), False
    result = sorted(entries + [new], key=lambda e: parse_tag(e["version"]), reverse=True)
    validate_upgrade_versions(result)
    return result, True


def release_date_from_timestamp(published_at, tz_hours=8):
    """GitHub returns UTC; the feed has always used China local dates."""
    ts = datetime.datetime.fromisoformat(published_at.replace("Z", "+00:00"))
    return (ts + datetime.timedelta(hours=tz_hours)).date().isoformat()


# --------------------------------------------------------------------------
# text stamping
# --------------------------------------------------------------------------

def _replace_once(pattern, text, repl, what):
    matches = pattern.findall(text)
    if len(matches) != 1:
        raise ReleaseError(f"expected exactly one {what}, found {len(matches)}")
    return pattern.sub(repl, text, count=1)


def stamp_install_script(text, tag):
    parse_tag(tag)
    return _replace_once(INSTALL_VERSION_RE, text,
                         lambda m: f"RAINBOND_VERSION=${{VERSION:-'{tag}'}}",
                         "RAINBOND_VERSION default line")


def bump_chart(chart_yaml, values_yaml, tag):
    """Point the chart at tag. Never moves it backwards, so re-running an older
    release (rollback of the upgrade feed) leaves the chart alone."""
    version = chart_version(tag)
    current = CHART_VERSION_RE.search(chart_yaml)
    if current:
        cur = tuple(int(x) for x in re.findall(r"\d+", current.group(0))[:3])
        if cur >= parse_tag(tag):
            return chart_yaml, values_yaml
    chart_yaml = _replace_once(CHART_VERSION_RE, chart_yaml,
                               lambda m: f"version: {version}", "Chart.yaml version")
    values_yaml = _replace_once(VALUES_INSTALL_VERSION_RE, values_yaml,
                                lambda m: f"{m.group(1)}installVersion: {tag}",
                                "values.yaml installVersion")
    return chart_yaml, values_yaml


# --------------------------------------------------------------------------
# docs/versions.md
# --------------------------------------------------------------------------

CURRENT_ROW_RE = re.compile(
    r"^\| v(\d+)\.(\d+)\.x-release (\s*)\| (\[https://www\.rainbond\.com/docs\]\(https://www\.rainbond\.com/docs\)) \| (.*) \|$",
    re.M)
PAST_HEADER_RE = re.compile(r"^## 过往版本.*?\n(?:.*\n)*?\|[ -]+\|[ -]+\|[ -]+\|\n", re.M)


def roll_docs_versions(text, tag):
    """When tag starts a new minor line, make it current and archive the old
    one. Returns (text, changed)."""
    major, minor, _ = parse_tag(tag)
    m = CURRENT_ROW_RE.search(text)
    if not m:
        raise ReleaseError("current version row not found in docs/versions.md")
    cur_major, cur_minor = int(m.group(1)), int(m.group(2))
    if (major, minor) <= (cur_major, cur_minor):
        return text, False
    new_current = (f"| v{major}.{minor}.x-release {m.group(3)}| {m.group(4)} | {m.group(5)} |")
    text = text[:m.start()] + new_current + text[m.end():]
    archived = (f"| v{cur_major}.{cur_minor}.x-release | https://v{cur_major}.{cur_minor}.rainbond.com/docs "
                f"| [版本变更日志](https://github.com/goodrain/rainbond/releases/tag/v{cur_major}.{cur_minor}.0-release) |\n")
    h = PAST_HEADER_RE.search(text)
    if not h:
        raise ReleaseError("past versions table not found in docs/versions.md")
    return text[:h.end()] + archived + text[h.end():], True



# --------------------------------------------------------------------------
# E2E gate
# --------------------------------------------------------------------------

def check_e2e_provenance(prov, expected_sources):
    """Problems that stop a release: the E2E run must be a green full run of
    exactly the commits that are about to be built (see rainbond-e2e-tests
    e2e-provenance.json)."""
    problems = []
    if prov.get("result") != "success":
        problems.append(f"latest E2E result is {prov.get('result')!r}, not 'success'")
    if prov.get("suite") != "full":
        problems.append(f"latest E2E suite is {prov.get('suite')!r}, not 'full'")
    sources = prov.get("sources") or {}
    for repo, sha in sorted(expected_sources.items()):
        tested = sources.get(repo)
        if not tested:
            problems.append(f"E2E did not record which {repo} commit it tested")
        elif tested != sha:
            problems.append(f"{repo}: E2E tested {tested[:12]} but main is now {sha[:12]}; rerun E2E")
    return problems

# --------------------------------------------------------------------------
# registries
# --------------------------------------------------------------------------

def _http_get(url, headers=None, timeout=30):
    req = urllib.request.Request(url, headers=headers or {})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return resp.read(), dict(resp.headers)


def _parse_www_authenticate(value):
    scheme, _, params = value.partition(" ")
    if scheme.lower() != "bearer":
        raise ReleaseError(f"unsupported registry auth scheme {scheme!r}")
    return dict(re.findall(r'(\w+)="([^"]*)"', params))


def _split_image(image):
    host, _, path = image.partition("/")
    registry = "registry-1.docker.io" if host == "docker.io" else host
    return registry, path


def fetch_manifest(image, tag, get=_http_get):
    """Anonymous pull of a manifest, following the standard bearer-token
    challenge (works for Docker Hub and public Aliyun repositories)."""
    registry, repo = _split_image(image)
    url = f"https://{registry}/v2/{repo}/manifests/{tag}"
    headers = {"Accept": MANIFEST_ACCEPT}
    try:
        body, _ = get(url, headers)
    except urllib.error.HTTPError as err:
        if err.code != 401:
            raise
        params = _parse_www_authenticate(err.headers.get("WWW-Authenticate", ""))
        realm = params.pop("realm")
        params.setdefault("scope", f"repository:{repo}:pull")
        query = "&".join(f"{k}={v}" for k, v in params.items())
        token_body, _ = get(f"{realm}?{query}")
        token = json.loads(token_body)
        headers["Authorization"] = "Bearer " + (token.get("token") or token.get("access_token"))
        body, _ = get(url, headers)
    return json.loads(body)


def manifest_arches(manifest):
    if "manifests" not in manifest:
        return set()
    return {m.get("platform", {}).get("architecture") for m in manifest["manifests"]} - {None, "unknown"}


def verify_images(tag, images=RELEASE_IMAGES, fetch=fetch_manifest):
    problems = []
    for image in images:
        try:
            arches = manifest_arches(fetch(image, tag))
        except urllib.error.HTTPError as err:
            problems.append(f"{image}:{tag} not found (HTTP {err.code})")
            continue
        missing = REQUIRED_ARCHES - arches
        if missing:
            problems.append(f"{image}:{tag} is missing {', '.join(sorted(missing))}")
    return problems


# --------------------------------------------------------------------------
# CLI
# --------------------------------------------------------------------------

def _read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def _write(path, text):
    with open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)


def cmd_check_tag(args):
    major, minor, patch = parse_tag(args.tag)
    print(f"tag={args.tag}")
    print(f"chart_version={major}.{minor}.{patch}")
    print(f"minor_line=v{major}.{minor}")


def cmd_verify_images(args):
    problems = verify_images(args.tag)
    for p in problems:
        print(f"::error::{p}")
    if problems:
        raise ReleaseError(f"{len(problems)} release image(s) are not multi-arch")
    print(f"all {len(RELEASE_IMAGES)} images for {args.tag} have {', '.join(sorted(REQUIRED_ARCHES))}")


def cmd_upgrade_versions(args):
    entries = json.loads(_read(args.file))
    validate_upgrade_versions(entries)
    date = args.date or release_date_from_timestamp(args.published_at)
    entries, changed = add_upgrade_version(entries, args.tag, date)
    _write(args.output or args.file, dump_upgrade_versions(entries))
    print(f"{args.tag} {'added' if changed else 'already present'} ({date})")


def cmd_install_script(args):
    _write(args.output, stamp_install_script(_read(args.source), args.tag))


def cmd_chart_bump(args):
    chart, values = bump_chart(_read(f"{args.chart_dir}/Chart.yaml"),
                               _read(f"{args.chart_dir}/values.yaml"), args.tag)
    _write(f"{args.chart_dir}/Chart.yaml", chart)
    _write(f"{args.chart_dir}/values.yaml", values)


def cmd_docs_versions(args):
    text, changed = roll_docs_versions(_read(args.file), args.tag)
    if changed:
        _write(args.file, text)
    print(f"changed={'true' if changed else 'false'}")


def cmd_verify_url(args):
    want = hashlib.sha256(open(args.file, "rb").read()).hexdigest()
    got = None
    for attempt in range(args.retries):
        body, _ = _http_get(args.url, {"Cache-Control": "no-cache"})
        got = hashlib.sha256(body).hexdigest()
        if got == want:
            print(f"{args.url} matches {args.file} ({want[:12]})")
            return
        if attempt + 1 < args.retries:
            time.sleep(args.interval)
    raise ReleaseError(f"{args.url} serves {got[:12]}, expected {want[:12]} "
                       "(CDN may still be caching the old file)")


def cmd_check_e2e(args):
    expected = {}
    for item in args.source:
        repo, _, sha = item.partition("=")
        if not repo or not sha:
            raise ReleaseError(f"--source must be repo=sha, got {item!r}")
        expected[repo] = sha
    with open(args.file, encoding="utf-8") as f:
        prov = json.load(f)
    problems = check_e2e_provenance(prov, expected)
    for p in problems:
        print(f"::error::{p}")
    if problems:
        raise ReleaseError("E2E does not cover this release")
    print(f"E2E run {prov.get('run_id')} ({prov.get('tested_at')}) covers {', '.join(sorted(expected))}")


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("check-tag")
    s.add_argument("tag")
    s.set_defaults(func=cmd_check_tag)

    s = sub.add_parser("verify-images")
    s.add_argument("tag")
    s.set_defaults(func=cmd_verify_images)

    s = sub.add_parser("upgrade-versions")
    s.add_argument("tag")
    s.add_argument("--file", default="hack/release/upgrade-versions.json")
    s.add_argument("--output")
    g = s.add_mutually_exclusive_group(required=True)
    g.add_argument("--date", help="release date, YYYY-MM-DD")
    g.add_argument("--published-at", help="GitHub release published_at (UTC)")
    s.set_defaults(func=cmd_upgrade_versions)

    s = sub.add_parser("install-script")
    s.add_argument("tag")
    s.add_argument("--source", required=True)
    s.add_argument("--output", required=True)
    s.set_defaults(func=cmd_install_script)

    s = sub.add_parser("chart-bump")
    s.add_argument("tag")
    s.add_argument("--chart-dir", required=True)
    s.set_defaults(func=cmd_chart_bump)

    s = sub.add_parser("docs-versions")
    s.add_argument("tag")
    s.add_argument("--file", required=True)
    s.set_defaults(func=cmd_docs_versions)

    s = sub.add_parser("verify-url")
    s.add_argument("url")
    s.add_argument("--file", required=True)
    s.add_argument("--retries", type=int, default=10)
    s.add_argument("--interval", type=int, default=30)
    s.set_defaults(func=cmd_verify_url)

    s = sub.add_parser("check-e2e")
    s.add_argument("--file", required=True, help="e2e-provenance.json")
    s.add_argument("--source", action="append", default=[], help="repo=sha, repeatable")
    s.set_defaults(func=cmd_check_e2e)

    args = p.parse_args(argv)
    try:
        args.func(args)
    except ReleaseError as err:
        print(f"::error::{err}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
