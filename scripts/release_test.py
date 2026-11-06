#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""uv run scripts/release_test.py"""

import unittest

from release import (
    ARCHS,
    VERSION,
    Release,
    ReleaseError,
    can_resume,
    developer_id,
    is_prerelease,
    notarization_result,
    tag_action,
)


class TagTests(unittest.TestCase):
    def test_creates_a_missing_tag(self):
        self.assertEqual(tag_action("v0.1.0", None, "abc", pushed=False), "create")

    def test_keeps_a_tag_at_head(self):
        self.assertEqual(tag_action("v0.1.0", "abc", "abc", pushed=True), "keep")

    def test_moves_an_unpushed_tag_to_head(self):
        self.assertEqual(tag_action("v0.1.0", "old", "abc", pushed=False), "move")

    def test_never_moves_a_pushed_tag(self):
        with self.assertRaisesRegex(ReleaseError, "v0.1.0 is already pushed"):
            tag_action("v0.1.0", "old", "abc", pushed=True)


class VersionTests(unittest.TestCase):
    def test_accepts_releases_and_candidates(self):
        for version in ["0.1.0", "10.2.33", "0.1.1-rc.1", "1.0.0-rc.12"]:
            self.assertTrue(VERSION.fullmatch(version), version)
        for version in ["0.1", "v0.1.0", "0.1.1-rc", "0.1.1-beta.1", "0.1.1-rc.1x"]:
            self.assertFalse(VERSION.fullmatch(version), version)

    def test_candidates_are_prereleases(self):
        self.assertTrue(is_prerelease("0.1.1-rc.1"))
        self.assertFalse(is_prerelease("0.1.1"))


class ResumeTests(unittest.TestCase):
    STATE = {"version": "0.1.0", "commit": "abc", "build": 42, "submissions": {"arm64": "id"}}

    def test_resumes_the_build_of_the_tagged_commit(self):
        self.assertTrue(can_resume(self.STATE, "0.1.0", "abc"))

    def test_rebuilds_when_anything_differs(self):
        self.assertFalse(can_resume(None, "0.1.0", "abc"))
        self.assertFalse(can_resume(self.STATE, "0.1.1", "abc"))
        self.assertFalse(can_resume(self.STATE, "0.1.0", "moved"))
        self.assertFalse(can_resume(self.STATE, "0.1.0", None))


class ArchTests(unittest.TestCase):
    def test_builds_every_supported_architecture(self):
        self.assertEqual(sorted(ARCHS), ["arm64", "x86_64"])

    def test_archives_and_apps_are_named_per_architecture(self):
        release = Release("1.2.3")
        for arch in ARCHS:
            self.assertEqual(release.dmg(arch).name, f"Code-1.2.3-{arch}.dmg")
            self.assertIn(arch, release.app(arch).parts)
            self.assertNotEqual(release.app(arch), release.app("other" if arch != "other" else "x86_64"))


class NotarizationTests(unittest.TestCase):
    def test_reads_status_and_id(self):
        self.assertEqual(notarization_result('{"id": "123", "status": "Invalid"}'), ("Invalid", "123"))

    def test_unreadable_output_is_unknown(self):
        for text in ["", "Error: network", "[]", "{}"]:
            with self.subTest(text=text):
                self.assertEqual(notarization_result(text)[0], "unknown")


class ToolOutputTests(unittest.TestCase):
    def test_developer_id_for_the_team(self):
        text = '''  1) AAA "Apple Development: Someone (ZZZ)"
  2) BBB "Developer ID Application: Someone (OTHERTEAM)"
  3) CCC "Developer ID Application: Someone (DP5STKBRSS)"
     3 valid identities found'''
        self.assertEqual(developer_id(text, "DP5STKBRSS"), "Developer ID Application: Someone (DP5STKBRSS)")
        self.assertIsNone(developer_id(text, "NOPE"))


if __name__ == "__main__":
    unittest.main()
