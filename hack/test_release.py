# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0

"""Check stable release versions before build and publication."""

import os
from pathlib import Path
import subprocess
import unittest


SCRIPT = Path(__file__).with_name("release.sh").resolve()


class ReleaseTest(unittest.TestCase):
    def setUp(self):
        self.root = SCRIPT.parent.parent
        self.env = dict(os.environ)

    def run_release(self, *args):
        return subprocess.run(["bash", str(SCRIPT), *args], cwd=self.root,
                              env=self.env, text=True, capture_output=True)

    def test_only_stable_release_versions_are_accepted(self):
        for tag, valid in [("v1.2.3", True), ("v0.0.0", True), ("v01.2.3", False),
                           ("v1.2.3-rc.1", False), ("main", False), ("v1.2.3;pwd", False)]:
            with self.subTest(tag=tag):
                self.env["RELEASE_TAG"] = tag
                result = self.run_release("version")
                self.assertEqual(result.returncode == 0, valid, result.stderr)
                if valid:
                    self.assertEqual(result.stdout.strip(), tag[1:])


if __name__ == "__main__":
    unittest.main()
