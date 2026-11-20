import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import validate_test_manifest as validator


ANCESTORS = ("ordinary", ".worktrees", ".claude", ".git", "vendor", "_output", "node_modules", "dist")


def create_fixture(root, registered=True):
    test_file = root / "pkg" / "sample_test.go"
    test_file.parent.mkdir(parents=True)
    marker = "// capability" + "_id: rainbond.sample\n"
    test_file.write_text("package sample\n" + marker, encoding="utf-8")
    (root / "pkg" / "sample.go").write_text("package sample\n", encoding="utf-8")
    capability = {
        "id": "rainbond.sample",
        "title": "Sample",
        "interface_type": "other",
        "interface": "sample",
        "code_paths": ["pkg/sample.go"],
        "tests": [{"path": "pkg/sample_test.go"}],
        "test_type": "regression",
        "status": "active",
    }
    manifest = root / "test-manifest.json"
    manifest.write_text(json.dumps({
        "version": 1, "repo": "sample", "capabilities": [capability] if registered else [],
    }), encoding="utf-8")
    return manifest


def run_cli(root):
    return subprocess.run(
        [sys.executable, validator.__file__, "--repo-root", str(root)], capture_output=True, text=True)


# capability_id: rainbond.test-manifest.repo-relative-scan
class ManifestRootBoundaryTests(unittest.TestCase):
    def test_external_ignored_ancestors_do_not_hide_markers_or_change_validation(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            for ancestor in ANCESTORS:
                with self.subTest(ancestor=ancestor):
                    root = Path(temp_dir) / ancestor / "repo"
                    manifest = create_fixture(root)
                    self.assertEqual(validator.collect_marked_tests(root), {"pkg/sample_test.go": ["rainbond.sample"]})
                    self.assertEqual(validator.validate_manifest(root, manifest), [])

    def test_cli_missing_registration_fails_identically_under_every_ancestor(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            errors = []
            for ancestor in ANCESTORS:
                with self.subTest(ancestor=ancestor):
                    root = Path(temp_dir) / ancestor / "repo"
                    create_fixture(root, registered=False)
                    result = run_cli(root)
                    self.assertEqual(result.returncode, 1)
                    self.assertIn("managed test references unknown capability_id", result.stderr)
                    errors.append(result.stderr)
            self.assertEqual(len(set(errors)), 1)

    def test_repository_internal_exclusions_keep_the_original_go_policy(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            create_fixture(root)
            for directory in (".git", "vendor", "_output", "node_modules", "dist"):
                ignored = root / directory / "ignored_test.go"
                ignored.parent.mkdir()
                ignored.write_text("// capability" + "_id: rainbond.ignored\n", encoding="utf-8")
            self.assertEqual(validator.collect_marked_tests(root), {"pkg/sample_test.go": ["rainbond.sample"]})

    def test_internal_worktrees_and_claude_are_still_scanned_in_go(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            create_fixture(root)
            for directory in (".worktrees", ".claude"):
                included = root / directory / "included_test.go"
                included.parent.mkdir()
                included.write_text("// capability" + "_id: rainbond.included\n", encoding="utf-8")
            self.assertEqual(set(validator.collect_marked_tests(root)), {
                "pkg/sample_test.go", ".worktrees/included_test.go", ".claude/included_test.go",
            })

    def test_success_cli_reports_a_nonzero_scan_under_excluded_ancestor(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "vendor" / "repo"
            create_fixture(root)
            result = run_cli(root)
            self.assertEqual(result.returncode, 0)
            self.assertIn("marked test files: 1", result.stdout)


# capability_id: rainbond.test-manifest.nonempty-marker-control
class ManifestScanCompletenessTests(unittest.TestCase):
    def test_existing_managed_marker_cannot_pass_an_unexpected_empty_scan(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            manifest = create_fixture(root)
            with mock.patch.object(validator, "collect_marked_tests", return_value={}):
                errors = validator.validate_manifest(root, manifest)
            self.assertTrue(any("scan returned no files" in error for error in errors))

    def test_truly_empty_repository_remains_valid(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            manifest = root / "test-manifest.json"
            manifest.write_text(json.dumps({"version": 1, "repo": "empty", "capabilities": []}), encoding="utf-8")
            self.assertEqual(validator.validate_manifest(root, manifest), [])


if __name__ == "__main__":
    unittest.main()
