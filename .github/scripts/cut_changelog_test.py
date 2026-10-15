#!/usr/bin/env python3
"""cut_changelog：无实质变更时不改写文件。"""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

import cut_changelog as cc


class CutChangelogTest(unittest.TestCase):
    def test_promote_no_change_when_version_section_ready(self) -> None:
        text = """# log

## [Unreleased]


## [1.1.5] - 2026-10-15

### 变更
- 已有说明
"""
        preamble, sections = cc.parse(text)
        updated, changed = cc.promote(sections, "1.1.5", "2026-10-15")
        self.assertFalse(changed)
        target = next(s for s in updated if s["title"] == "1.1.5")
        self.assertEqual(target["suffix"], "- 2026-10-15")
        self.assertTrue(cc.has_notes(target["lines"]))

    def test_apply_skips_whitespace_only_rewrite(self) -> None:
        original = """# log

## [Unreleased]


## [1.1.5] - 2026-10-15

### 变更
- 已有说明
"""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            changelog = root / "CHANGELOG.md"
            notes = root / "release-notes.md"
            changelog.write_text(original, encoding="utf-8", newline="\n")
            preamble, sections = cc.parse(changelog.read_text(encoding="utf-8"))
            updated, changed = cc.promote(sections, "1.1.5", "2026-10-15")
            self.assertFalse(changed)
            notes.write_text(cc.notes_for(updated, "1.1.5"), encoding="utf-8", newline="\n")
            if changed:
                changelog.write_text(cc.render(preamble, updated), encoding="utf-8", newline="\n")
            self.assertEqual(changelog.read_text(encoding="utf-8"), original)

    def test_promote_moves_unreleased_notes(self) -> None:
        text = """# log

## [Unreleased]

### 变更
- 新功能

## [1.1.4] - 2026-10-14

### 变更
- 旧说明
"""
        preamble, sections = cc.parse(text)
        updated, changed = cc.promote(sections, "1.1.6", "2026-10-15")
        self.assertTrue(changed)
        target = next(s for s in updated if s["title"] == "1.1.6")
        self.assertEqual(target["suffix"], "- 2026-10-15")
        self.assertTrue(cc.has_notes(target["lines"]))
        unreleased = next(s for s in updated if s["title"] == "Unreleased")
        self.assertFalse(cc.has_notes(unreleased["lines"]))


if __name__ == "__main__":
    unittest.main()
