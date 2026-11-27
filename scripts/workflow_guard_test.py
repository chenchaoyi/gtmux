"""Tests for harness-agnostic AGENTS.md and cross-harness OpenSpec workflow guards."""
from pathlib import Path
import tempfile
import unittest

import workflow_guard

ROOT = Path(__file__).resolve().parents[1]


def seed_valid_repo(root: Path) -> None:
    (root / "AGENTS.md").write_text("# Repo guide\n", encoding="utf-8")
    for pointer in workflow_guard.REQUIRED_POINTERS:
        (root / pointer).write_text("<!-- thin pointer -->\n@AGENTS.md\n", encoding="utf-8")
    for skill_root in workflow_guard.SKILL_ROOTS:
        for skill in workflow_guard.REQUIRED_OPENSPEC_SKILLS:
            skill_file = root / skill_root / skill / "SKILL.md"
            skill_file.parent.mkdir(parents=True, exist_ok=True)
            skill_file.write_text(f"---\nname: {skill}\n---\n", encoding="utf-8")
        for ref in workflow_guard.REQUIRED_OPENSPEC_REFERENCES:
            ref_file = root / skill_root / "openspec" / "references" / ref
            ref_file.parent.mkdir(parents=True, exist_ok=True)
            ref_file.write_text(f"# {ref}\n", encoding="utf-8")


class WorkflowGuardTests(unittest.TestCase):
    def test_real_repo_passes_workflow_guard(self):
        self.assertEqual(workflow_guard.check_repo(ROOT), [])

    def test_rejects_missing_symlinked_or_bloated_instructions(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            seed_valid_repo(root)
            (root / "AGENTS.md").unlink()
            self.assertTrue(any("AGENTS.md is missing" in e for e in workflow_guard.check_repo(root)))

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            seed_valid_repo(root)
            (root / "CLAUDE.md").unlink()
            (root / "CLAUDE.md").symlink_to("AGENTS.md")
            self.assertTrue(any("CLAUDE.md" in e and "symbolic link" in e for e in workflow_guard.check_repo(root)))

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            seed_valid_repo(root)
            (root / "CLAUDE.md").write_text("# Read AGENTS.md manually\n", encoding="utf-8")
            self.assertTrue(any("CLAUDE.md must import @AGENTS.md" in e for e in workflow_guard.check_repo(root)))

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            seed_valid_repo(root)
            (root / "GEMINI.md").write_text("@AGENTS.md\n## Duplicated section\n", encoding="utf-8")
            self.assertTrue(any("GEMINI.md" in e and "## " in e for e in workflow_guard.check_repo(root)))

    def test_rejects_missing_or_empty_cross_harness_openspec_skills(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            seed_valid_repo(root)
            (root / ".agents/skills/openspec-propose/SKILL.md").unlink()
            (root / ".claude/skills/openspec-archive-change/SKILL.md").write_text("   \n", encoding="utf-8")
            errs = workflow_guard.check_repo(root)
            self.assertTrue(any(".agents/skills/openspec-propose/SKILL.md" in e for e in errs), errs)
            self.assertTrue(any(".claude/skills/openspec-archive-change/SKILL.md" in e for e in errs), errs)


if __name__ == "__main__":
    unittest.main()
