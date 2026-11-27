#!/usr/bin/env python3
"""Enforce harness-agnostic repo instructions (AGENTS.md) and cross-harness OpenSpec 2.x skills."""
from pathlib import Path
import sys

REQUIRED_POINTERS = ("CLAUDE.md", "GEMINI.md")
MAX_POINTER_LINES = 10
REQUIRED_OPENSPEC_SKILLS = (
    "openspec",
    "openspec-propose",
    "openspec-apply-change",
    "openspec-archive-change",
    "openspec-explore",
    "openspec-sync-specs",
)
REQUIRED_OPENSPEC_REFERENCES = (
    "answers.md",
    "where-things-stand.md",
    "without-skills.md",
)
SKILL_ROOTS = (".agents/skills", ".claude/skills")


def check_repo(root: Path) -> list[str]:
    errors: list[str] = []
    agents = root / "AGENTS.md"
    if agents.is_symlink():
        errors.append("AGENTS.md must be a regular file, not a symbolic link")
    elif not agents.is_file():
        errors.append("AGENTS.md is missing (canonical repo instructions)")
    else:
        text = agents.read_text(encoding="utf-8", errors="replace").strip()
        if not text:
            errors.append("AGENTS.md is empty")

    for name in REQUIRED_POINTERS:
        p = root / name
        if p.is_symlink():
            errors.append(f"{name} must be a thin regular-file @AGENTS.md pointer, not a symbolic link")
            continue
        if not p.is_file():
            errors.append(f"{name} is missing (expected thin @AGENTS.md import pointer)")
            continue
        lines = [ln.strip() for ln in p.read_text(encoding="utf-8", errors="replace").splitlines() if ln.strip()]
        if "@AGENTS.md" not in lines:
            errors.append(f"{name} must import @AGENTS.md on its own line")
        if len(lines) > MAX_POINTER_LINES:
            errors.append(f"{name} has {len(lines)} non-empty lines (max {MAX_POINTER_LINES}); keep rules in AGENTS.md")
        if any(ln.startswith("## ") for ln in lines):
            errors.append(f"{name} must not define '## ' sections; keep substantive rules in AGENTS.md")

    for skill_root in SKILL_ROOTS:
        for skill in REQUIRED_OPENSPEC_SKILLS:
            skill_dir = root / skill_root / skill
            skill_file = skill_dir / "SKILL.md"
            if skill_dir.is_symlink() or skill_file.is_symlink():
                errors.append(f"{skill_root}/{skill}/SKILL.md must be a regular file, not a symbolic link")
            elif not skill_file.is_file():
                errors.append(f"missing OpenSpec skill: {skill_root}/{skill}/SKILL.md")
            elif not skill_file.read_text(encoding="utf-8", errors="replace").strip():
                errors.append(f"empty OpenSpec skill: {skill_root}/{skill}/SKILL.md")
        for ref in REQUIRED_OPENSPEC_REFERENCES:
            ref_file = root / skill_root / "openspec" / "references" / ref
            if ref_file.is_symlink():
                errors.append(f"{skill_root}/openspec/references/{ref} must be a regular file, not a symbolic link")
            elif not ref_file.is_file() or not ref_file.read_text(encoding="utf-8", errors="replace").strip():
                errors.append(f"missing or empty OpenSpec reference: {skill_root}/openspec/references/{ref}")

    return errors


def main() -> int:
    root = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else Path.cwd().resolve()
    errors = check_repo(root)
    if errors:
        for err in errors:
            print(f"workflow-guard: {err}", file=sys.stderr)
        return 1
    print("workflow-guard: OK — AGENTS.md canonical, CLAUDE.md/GEMINI.md thin pointers, cross-harness OpenSpec 2.x skills present")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
