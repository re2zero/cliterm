#!/usr/bin/env python3
"""
Task Complexity Detection.

Evaluates task requirements to determine if a task is simple
or complex, guiding the workflow automation strategy.

Simple tasks: auto-generate PRD, direct execution.
Complex tasks: brainstorm Q&A, cowork dispatch.
"""

from __future__ import annotations

import re
import subprocess
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class ComplexityResult:
    level: str
    reason: str
    estimated_files: int
    estimated_minutes: int

    @property
    def is_simple(self) -> bool:
        return self.level == "simple"

    @property
    def needs_brainstorm_qa(self) -> bool:
        return self.level == "complex"


COMPLEXITY_INDICATORS = [
    r"\bimplement\b",
    r"\bcreate\b",
    r"\bdesign\b",
    r"\barchitect\b",
    r"\brefactor\b.*\bmultiple\b",
    r"\bmigrate\b",
    r"\bintegrate\b",
]

SIMPLE_INDICATORS = [
    r"\bfix\b",
    r"\bpatch\b",
    r"\bupdate\b",
    r"\btypo\b",
    r"\bbug\b",
    r"\bminor\b",
    r"\btrivial\b",
    r"\bcleanup\b",
    r"\brename\b",
]

FILE_PATTERN_ESTIMATES = {
    "model": 2,
    "controller": 1,
    "service": 2,
    "repository": 1,
    "test": 2,
    "config": 1,
    "component": 2,
    "page": 2,
    "route": 1,
    "middleware": 1,
    "schema": 1,
    "validator": 1,
    "dto": 1,
    "entity": 1,
}

MINUTES_PER_FILE = 3


def detect_complexity(requirement: str) -> ComplexityResult:
    """Detect task complexity from requirement description.

    Args:
        requirement: Task requirement description.

    Returns:
        ComplexityResult with level, reason, and estimates.
    """
    text = requirement.strip()
    text_lower = text.lower()

    description_length = len(text)

    complex_hits = sum(
        1 for pattern in COMPLEXITY_INDICATORS if re.search(pattern, text_lower)
    )
    simple_hits = sum(
        1 for pattern in SIMPLE_INDICATORS if re.search(pattern, text_lower)
    )

    estimated_files = _estimate_file_count(text_lower)
    estimated_minutes = max(estimated_files * MINUTES_PER_FILE, 5)

    if complex_hits > 0 and complex_hits > simple_hits:
        return ComplexityResult(
            level="complex",
            reason=(
                f"Complex indicators found ({complex_hits}): "
                f"description suggests significant new work"
            ),
            estimated_files=estimated_files,
            estimated_minutes=estimated_minutes,
        )

    if simple_hits > 0 and simple_hits >= complex_hits:
        if description_length < 100 and estimated_files <= 3:
            return ComplexityResult(
                level="simple",
                reason=(
                    f"Simple indicators found ({simple_hits}): "
                    f"short description, few files expected"
                ),
                estimated_files=estimated_files,
                estimated_minutes=estimated_minutes,
            )

    if description_length < 50 and estimated_files <= 2:
        return ComplexityResult(
            level="simple",
            reason="Short requirement with minimal file impact",
            estimated_files=estimated_files,
            estimated_minutes=estimated_minutes,
        )

    if estimated_files > 5 or estimated_minutes > 15:
        return ComplexityResult(
            level="complex",
            reason=(
                f"Large scope: ~{estimated_files} files, "
                f"~{estimated_minutes}min estimated"
            ),
            estimated_files=estimated_files,
            estimated_minutes=estimated_minutes,
        )

    if complex_hits == 0 and simple_hits == 0:
        if description_length < 80:
            return ComplexityResult(
                level="simple",
                reason="Short requirement with no complexity signals",
                estimated_files=estimated_files,
                estimated_minutes=estimated_minutes,
            )

    return ComplexityResult(
        level="complex",
        reason="Default: unclear complexity, treating as complex",
        estimated_files=estimated_files,
        estimated_minutes=estimated_minutes,
    )


def _estimate_file_count(text: str) -> int:
    """Estimate number of files from requirement text.

    Args:
        text: Lowercase requirement text.

    Returns:
        Estimated file count (minimum 1).
    """
    count = 1
    for keyword, estimate in FILE_PATTERN_ESTIMATES.items():
        if keyword in text:
            count += estimate
    return count


def detect_from_prd(prd_path: Path) -> ComplexityResult:
    """Detect complexity from an existing PRD file.

    Args:
        prd_path: Path to prd.md file.

    Returns:
        ComplexityResult based on PRD content.
    """
    if not prd_path.is_file():
        return ComplexityResult(
            level="complex",
            reason="No PRD found, defaulting to complex",
            estimated_files=0,
            estimated_minutes=0,
        )

    try:
        content = prd_path.read_text(encoding="utf-8")
    except (OSError, IOError):
        return ComplexityResult(
            level="complex",
            reason="Cannot read PRD, defaulting to complex",
            estimated_files=0,
            estimated_minutes=0,
        )

    return detect_complexity(content)


def get_repo_root() -> Path:
    """Resolve the repository root from current working directory."""
    result = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode == 0:
        return Path(result.stdout.strip())
    return Path.cwd()


def get_current_task_abs(repo_root: Path | None = None) -> Path | None:
    """Get current task directory absolute path.

    Self-contained implementation (no dependency on common.paths).

    Args:
        repo_root: Repository root path. Auto-detected if None.

    Returns:
        Absolute path to current task directory or None.
    """
    if repo_root is None:
        repo_root = get_repo_root()

    current_file = repo_root / ".trellis" / ".current-task"
    if not current_file.is_file():
        return None

    try:
        relative = current_file.read_text(encoding="utf-8").strip()
    except (OSError, IOError):
        return None

    if not relative:
        return None

    full_path = repo_root / relative
    return full_path if full_path.is_dir() else None
