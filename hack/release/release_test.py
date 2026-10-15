"""Unit tests for hack/release/release.py (stdlib only, no network).

Run: python3 -m unittest discover -s hack/release -p '*_test.py'
"""

import io
import json
import os
import unittest
import urllib.error

import release

HERE = os.path.dirname(os.path.abspath(__file__))


def _entry(tag, date="2026-01-01"):
    return release.upgrade_entry(tag, date)


class TagTest(unittest.TestCase):
    def test_parse_and_chart_version(self):
        self.assertEqual(release.parse_tag("v6.9.12-release"), (6, 9, 12))
        self.assertEqual(release.chart_version("v6.10.0-release"), "6.10.0")

    def test_rejects_non_release_tags(self):
        for tag in ["v6.9.12", "6.9.12-release", "v6.9-release", "v6.9.12-dev", "v6.9.12-release\n"]:
            with self.assertRaises(release.ReleaseError, msg=tag):
                release.parse_tag(tag)


class UpgradeVersionsTest(unittest.TestCase):
    def test_entry_matches_published_shape(self):
        e = _entry("v6.9.11-release", "2026-10-12")
        self.assertEqual(e["detail"]["body"],
                         "For update details, reference: "
                         "https://github.com/goodrain/rainbond/releases/tag/v6.9.11-release.")
        self.assertEqual(list(e["images"]), ["rbd-api", "rbd-app-ui", "rbd-chaos", "rbd-mq", "rbd-worker"])
        self.assertEqual(e["images"]["rbd-app-ui"],
                         "registry.cn-hangzhou.aliyuncs.com/goodrain/rainbond:v6.9.11-release")

    def test_add_prepends_newest(self):
        entries = [_entry("v6.9.11-release"), _entry("v6.9.10-release")]
        out, changed = release.add_upgrade_version(entries, "v6.9.12-release", "2026-11-01")
        self.assertTrue(changed)
        self.assertEqual([e["version"] for e in out],
                         ["v6.9.12-release", "v6.9.11-release", "v6.9.10-release"])

    def test_sorts_by_semver_not_string(self):
        entries = [_entry("v6.9.11-release")]
        out, _ = release.add_upgrade_version(entries, "v6.10.0-release", "2026-11-01")
        self.assertEqual(out[0]["version"], "v6.10.0-release")

    def test_rerun_same_release_is_noop(self):
        entries = [_entry("v6.9.12-release", "2026-11-01"), _entry("v6.9.11-release")]
        out, changed = release.add_upgrade_version(entries, "v6.9.12-release", "2026-11-01")
        self.assertFalse(changed)
        self.assertEqual(out, entries)

    def test_conflicting_existing_entry_is_refused(self):
        entries = [_entry("v6.9.12-release", "2026-11-01")]
        with self.assertRaises(release.ReleaseError):
            release.add_upgrade_version(entries, "v6.9.12-release", "2026-11-02")

    def test_foreign_image_is_refused(self):
        bad = _entry("v6.9.12-release")
        bad["images"]["rbd-api"] = "docker.io/attacker/rbd-api:v6.9.12-release"
        with self.assertRaises(release.ReleaseError):
            release.validate_upgrade_versions([bad])

    def test_unsorted_feed_is_refused(self):
        with self.assertRaises(release.ReleaseError):
            release.validate_upgrade_versions([_entry("v6.9.10-release"), _entry("v6.9.11-release")])

    def test_china_date_from_utc_timestamp(self):
        # 2026-10-11 20:00 UTC is already 10-12 in Beijing.
        self.assertEqual(release.release_date_from_timestamp("2026-10-11T20:00:00Z"), "2026-10-12")

    def test_checked_in_feed_is_valid_and_round_trips(self):
        path = os.path.join(HERE, "upgrade-versions.json")
        with open(path, encoding="utf-8") as f:
            raw = f.read()
        entries = json.loads(raw)
        release.validate_upgrade_versions(entries)
        self.assertEqual(release.dump_upgrade_versions(entries), raw)


class StampTest(unittest.TestCase):
    SCRIPT = "#!/bin/bash\n\nRAINBOND_VERSION=${VERSION:-'v6.9.7-release'}\nOTHER=1\n"

    def test_install_script_only_changes_version_line(self):
        out = release.stamp_install_script(self.SCRIPT, "v6.9.12-release")
        self.assertEqual(out, self.SCRIPT.replace("v6.9.7-release", "v6.9.12-release"))

    def test_install_script_requires_exactly_one_line(self):
        with self.assertRaises(release.ReleaseError):
            release.stamp_install_script("#!/bin/bash\n", "v6.9.12-release")
        with self.assertRaises(release.ReleaseError):
            release.stamp_install_script(self.SCRIPT + self.SCRIPT, "v6.9.12-release")

    def test_chart_bump(self):
        chart = "apiVersion: v2\nname: rainbond\nversion: 6.9.10\nappVersion: v6\n"
        values = "Cluster:\n  enableHA: false\n  installVersion: v6.9.10-release\n"
        c, v = release.bump_chart(chart, values, "v6.9.11-release")
        self.assertIn("\nversion: 6.9.11\n", c)
        self.assertIn("appVersion: v6\n", c)
        self.assertEqual(v, "Cluster:\n  enableHA: false\n  installVersion: v6.9.11-release\n")

    def test_chart_never_moves_backwards(self):
        chart = "name: rainbond\nversion: 6.9.12\n"
        values = "  installVersion: v6.9.12-release\n"
        for tag in ["v6.9.12-release", "v6.9.11-release"]:
            self.assertEqual(release.bump_chart(chart, values, tag), (chart, values), tag)

    def test_chart_bump_ignores_nested_version_keys(self):
        chart = "name: rainbond\nversion: 6.9.10\ndependencies:\n  - name: x\n    version: 1.0.0\n"
        values = "  installVersion: v6.9.10-release\n"
        c, _ = release.bump_chart(chart, values, "v6.9.11-release")
        self.assertIn("    version: 1.0.0\n", c)


DOCS = """## 当前版本（稳定版）

| 版本号             | 文档链接 | 版本变更                               |
|-----------------| -------------------------------------- |------------------------------------|
| v6.9.x-release | [https://www.rainbond.com/docs](https://www.rainbond.com/docs) | [版本变更日志](https://github.com/goodrain/rainbond/releases) |

## 过往版本（不再维护）

在这里你可以找到过往版本的 Rainbond 文档。

| 版本号          | 文档链接                                                     | 版本变更                                                     |
| --------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| v6.8.x-release | https://v6.8.rainbond.com/docs | [版本变更日志](https://github.com/goodrain/rainbond/releases/tag/v6.8.0-release) |
"""


class DocsVersionsTest(unittest.TestCase):
    def test_patch_release_does_not_touch_docs(self):
        text, changed = release.roll_docs_versions(DOCS, "v6.9.12-release")
        self.assertFalse(changed)
        self.assertEqual(text, DOCS)

    def test_new_minor_rolls_table(self):
        text, changed = release.roll_docs_versions(DOCS, "v6.10.0-release")
        self.assertTrue(changed)
        self.assertIn("| v6.10.x-release | [https://www.rainbond.com/docs]", text)
        archived = ("| v6.9.x-release | https://v6.9.rainbond.com/docs | "
                    "[版本变更日志](https://github.com/goodrain/rainbond/releases/tag/v6.9.0-release) |\n"
                    "| v6.8.x-release")
        self.assertIn(archived, text)

    def test_older_release_leaves_docs_alone(self):
        text, changed = release.roll_docs_versions(DOCS, "v6.8.9-release")
        self.assertFalse(changed)
        self.assertEqual(text, DOCS)


class VerifyImagesTest(unittest.TestCase):
    @staticmethod
    def _index(*arches):
        return {"manifests": [{"platform": {"architecture": a, "os": "linux"}} for a in arches]
                + [{"platform": {"architecture": "unknown", "os": "unknown"}}]}

    def test_all_multi_arch(self):
        fetch = lambda image, tag: self._index("amd64", "arm64")
        self.assertEqual(release.verify_images("v6.9.11-release", fetch=fetch), [])

    def test_single_arch_and_missing_are_reported(self):
        def fetch(image, tag):
            if image.endswith("/rbd-api"):
                return {"mediaType": "application/vnd.docker.distribution.manifest.v2+json"}
            if image.endswith("/rbd-mq"):
                raise urllib.error.HTTPError(image, 404, "not found", {}, io.BytesIO())
            return self._index("amd64", "arm64")
        problems = release.verify_images("v6.9.11-release", fetch=fetch)
        self.assertTrue(any("rbd-api" in p and "amd64, arm64" in p for p in problems))
        self.assertTrue(any("rbd-mq" in p and "HTTP 404" in p for p in problems))

    def test_bearer_challenge_is_followed(self):
        calls = []

        def get(url, headers=None):
            calls.append((url, dict(headers or {})))
            if url.startswith("https://auth.example/token"):
                return json.dumps({"token": "t0k"}).encode(), {}
            if "Authorization" not in (headers or {}):
                raise urllib.error.HTTPError(
                    url, 401, "unauthorized",
                    {"WWW-Authenticate": 'Bearer realm="https://auth.example/token",service="reg"'},
                    io.BytesIO())
            return json.dumps(self._index("amd64", "arm64")).encode(), {}

        m = release.fetch_manifest("docker.io/rainbond/rbd-api", "v1", get=get)
        self.assertEqual(release.manifest_arches(m), {"amd64", "arm64"})
        self.assertEqual(calls[0][0], "https://registry-1.docker.io/v2/rainbond/rbd-api/manifests/v1")
        self.assertIn("scope=repository:rainbond/rbd-api:pull", calls[1][0])
        self.assertEqual(calls[2][1]["Authorization"], "Bearer t0k")


if __name__ == "__main__":
    unittest.main()
