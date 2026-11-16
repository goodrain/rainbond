"""Exercise bounded release commands with a local fake registry client.

Run on Linux (GNU timeout): python3 -m unittest discover -s hack/release -p '*_test.py'
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("retry.sh")


class RetryTest(unittest.TestCase):
    def run_command(self, failures=0, hang=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            client = root / "registry-client"
            client.write_text(
                "#!/usr/bin/env python3\n"
                "import json, os, pathlib, subprocess, sys, time\n"
                "root = pathlib.Path(os.environ['TEST_ROOT'])\n"
                "state = root / 'attempts'\n"
                "attempt = int(state.read_text()) + 1 if state.exists() else 1\n"
                "state.write_text(str(attempt))\n"
                "(root / 'args').write_text(json.dumps(sys.argv[1:]))\n"
                "if os.environ['TEST_HANG'] == '1':\n"
                "    subprocess.Popen([sys.executable, '-c', \"import os, pathlib, time; time.sleep(1.2); (pathlib.Path(os.environ['TEST_ROOT']) / 'escaped').touch()\"])\n"
                "    time.sleep(30)\n"
                "sys.exit(42 if attempt <= int(os.environ['TEST_FAILURES']) else 0)\n"
            )
            client.chmod(0o755)
            result = subprocess.run(
                ["bash", str(SCRIPT), str(client), "image@sha256:abc", "tag with spaces"],
                env={**os.environ, "TEST_ROOT": directory,
                     "TEST_FAILURES": str(failures), "TEST_HANG": str(int(hang)),
                     "RETRY_TIMEOUT": "1s" if hang else "5s", "RETRY_DELAY": "0"},
                capture_output=True, text=True, timeout=10,
            )
            attempts = int((root / "attempts").read_text()) if (root / "attempts").exists() else 0
            args = json.loads((root / "args").read_text()) if (root / "args").exists() else []
            return result, attempts, args, (root / "escaped").exists()

    def test_success_runs_once_and_preserves_arguments(self):
        result, attempts, args, _ = self.run_command()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(attempts, 1)
        self.assertEqual(args, ["image@sha256:abc", "tag with spaces"])

    def test_transient_failures_recover_without_rebuilding(self):
        result, attempts, _, _ = self.run_command(failures=2)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(attempts, 3)
        self.assertEqual(result.stdout, "")

    def test_exhausted_retries_fail_the_job(self):
        result, attempts, _, _ = self.run_command(failures=5)
        self.assertEqual(result.returncode, 42, result.stderr)
        self.assertEqual(attempts, 3)

    def test_stalled_upload_is_killed_and_retried(self):
        result, attempts, _, escaped = self.run_command(hang=True)
        self.assertEqual(result.returncode, 124, result.stderr)
        self.assertEqual(attempts, 3)
        self.assertFalse(escaped)


if __name__ == "__main__":
    unittest.main()
